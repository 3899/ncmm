#!/usr/bin/env python3
"""
Small cloud-side coordinator for NetEase VIP invitation gift tokens.

This file intentionally uses only the Python standard library so it can be
deployed as a single script. It stores gift tokens and claim records in SQLite.

Environment variables:
  VIP_GIFT_DB_PATH           SQLite database path, default: vip_member_gift_cloud.sqlite3
  VIP_GIFT_CLOUD_TOKEN       Optional API token. If set, non-health endpoints
                             require Authorization: Bearer <token> or X-Api-Key.
  VIP_GIFT_HOST              Server host, default: 0.0.0.0
  VIP_GIFT_PORT              Server port, default: 3102
  VIP_GIFT_MIN_AVAILABLE_DAYS Minimum days needed before a token is served, default: 7

Routes:
  GET  /health
  GET  /stats
  GET  /claims/status?month=YYYY-MM&receiverUid=UID
  GET  /tokens/available?month=YYYY-MM&receiverUid=UID
  POST /tokens/upsert
  POST /claims/success
  POST /tokens/fail
  POST /maintenance/prune
  POST /maintenance/delete-test-data

The client should deduct available days by reporting the actual duration from
the invitation accept response. The service never assumes a fixed claim size.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
import sqlite3
import sys
import time
import traceback
from datetime import datetime
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import parse_qs, urlparse


APP_NAME = "vip-member-gift-cloud"
SCHEMA_VERSION = 1
DEFAULT_MIN_AVAILABLE_DAYS = 7
MAX_BODY_BYTES = 1024 * 1024

TERMINAL_TOKEN_STATUSES = {
    "depleted",
    "expired",
    "invalid",
    "exhausted",
    "unavailable",
}


def now_ms() -> int:
    return int(time.time() * 1000)


def current_month() -> str:
    return datetime.now().strftime("%Y-%m")


def parse_int(value: Any, default: int = 0) -> int:
    if value is None or value == "":
        return default
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def normalize_millis(value: Any) -> int:
    n = parse_int(value, 0)
    if n <= 0:
        return 0
    if n < 10_000_000_000:
        return n * 1000
    return n


def as_text(value: Any) -> str:
    if value is None:
        return ""
    return str(value).strip()


def token_hash(token: str) -> str:
    return hashlib.sha256(token.encode("utf-8")).hexdigest()


def receiver_hash(receiver_uid: str) -> str:
    return hashlib.sha256(receiver_uid.encode("utf-8")).hexdigest()


def vip_level_to_days(vip_level: Any) -> int:
    level = as_text(vip_level).lower().lstrip("v")
    if level in {"2", "3", "4"}:
        return 98
    if level in {"5", "6"}:
        return 128
    if level == "7":
        return 258
    return 0


def json_dumps(data: Any) -> bytes:
    return json.dumps(data, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def parse_month(value: Any) -> str:
    month = as_text(value)
    if len(month) == 7 and month[4] == "-" and month[:4].isdigit() and month[5:].isdigit():
        return month
    return current_month()


def row_to_dict(row: sqlite3.Row | None) -> dict[str, Any] | None:
    if row is None:
        return None
    return {key: row[key] for key in row.keys()}


def public_token_row(row: sqlite3.Row | dict[str, Any] | None, include_token: bool) -> dict[str, Any] | None:
    if row is None:
        return None
    data = dict(row)
    if not include_token:
        data.pop("token", None)
    return data


def mask_uid(uid: str) -> str:
    uid = as_text(uid)
    if len(uid) <= 4:
        return "***"
    return uid[:2] + "***" + uid[-2:]


AVAILABLE_TOKEN_FIELDS = {"token_hash", "token", "month", "available_days", "source", "donor_uid"}


def available_token_row(row: sqlite3.Row | dict[str, Any] | None) -> dict[str, Any] | None:
    if row is None:
        return None
    data = dict(row)
    result = {k: data[k] for k in AVAILABLE_TOKEN_FIELDS if k in data}
    if "donor_uid" in result:
        result["donor_uid"] = mask_uid(result["donor_uid"])
    return result


class VipGiftStore:
    def __init__(self, db_path: str, min_available_days: int) -> None:
        self.db_path = db_path
        self.min_available_days = min_available_days
        self.failure_expire_ms = int(os.environ.get("VIP_GIFT_FAILURE_EXPIRE_MS", 3600 * 1000))
        self.init_db()

    def connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.db_path, timeout=30)
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA journal_mode=WAL")
        conn.execute("PRAGMA foreign_keys=ON")
        return conn

    def init_db(self) -> None:
        os.makedirs(os.path.dirname(os.path.abspath(self.db_path)), exist_ok=True)
        with self.connect() as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS meta (
                    key TEXT PRIMARY KEY,
                    value TEXT NOT NULL
                );

                CREATE TABLE IF NOT EXISTS vip_gift_tokens (
                    token_hash TEXT PRIMARY KEY,
                    token TEXT NOT NULL,
                    donor_uid TEXT NOT NULL DEFAULT '',
                    month TEXT NOT NULL,
                    vip_level TEXT NOT NULL DEFAULT '',
                    total_days INTEGER NOT NULL DEFAULT 0,
                    available_days INTEGER NOT NULL DEFAULT 0,
                    claimed_days INTEGER NOT NULL DEFAULT 0,
                    claim_count INTEGER NOT NULL DEFAULT 0,
                    expire_time_ms INTEGER NOT NULL DEFAULT 0,
                    status TEXT NOT NULL DEFAULT 'available',
                    source TEXT NOT NULL DEFAULT '',
                    metadata_json TEXT NOT NULL DEFAULT '{}',
                    created_at_ms INTEGER NOT NULL,
                    updated_at_ms INTEGER NOT NULL
                );

                CREATE TABLE IF NOT EXISTS vip_gift_claims (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    month TEXT NOT NULL,
                    receiver_uid TEXT NOT NULL,
                    token_hash TEXT NOT NULL,
                    donor_uid TEXT NOT NULL DEFAULT '',
                    record_id TEXT NOT NULL DEFAULT '',
                    duration INTEGER NOT NULL DEFAULT 0,
                    reward_name TEXT NOT NULL DEFAULT '',
                    accepted_at_ms INTEGER NOT NULL DEFAULT 0,
                    created_at_ms INTEGER NOT NULL,
                    updated_at_ms INTEGER NOT NULL,
                    UNIQUE(month, receiver_uid)
                );

                CREATE TABLE IF NOT EXISTS vip_gift_token_failures (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    month TEXT NOT NULL,
                    receiver_uid TEXT NOT NULL DEFAULT '',
                    token_hash TEXT NOT NULL,
                    reason TEXT NOT NULL DEFAULT '',
                    message TEXT NOT NULL DEFAULT '',
                    created_at_ms INTEGER NOT NULL,
                    UNIQUE(month, receiver_uid, token_hash, reason)
                );

                CREATE INDEX IF NOT EXISTS idx_tokens_available
                    ON vip_gift_tokens(month, status, available_days, expire_time_ms, updated_at_ms);
                CREATE INDEX IF NOT EXISTS idx_claims_receiver
                    ON vip_gift_claims(month, receiver_uid);
                CREATE INDEX IF NOT EXISTS idx_failures_receiver
                    ON vip_gift_token_failures(month, receiver_uid, token_hash);
                """
            )
            conn.execute(
                "INSERT OR REPLACE INTO meta(key, value) VALUES('schema_version', ?)",
                (str(SCHEMA_VERSION),),
            )

    def prune(self, month: str | None = None, min_available_days: int | None = None) -> dict[str, int]:
        target_month = parse_month(month) if month else current_month()
        min_days = min_available_days if min_available_days is not None else self.min_available_days
        now = now_ms()
        with self.connect() as conn:
            old_hashes = [
                row["token_hash"]
                for row in conn.execute(
                    """
                    SELECT token_hash
                    FROM vip_gift_tokens
                    WHERE available_days < ?
                       OR month < ?
                       OR (expire_time_ms > 0 AND expire_time_ms <= ?)
                    """,
                    (min_days, target_month, now),
                )
            ]
            if not old_hashes:
                return {"deletedTokens": 0, "deletedFailures": 0}
            placeholders = ",".join("?" for _ in old_hashes)
            token_deleted = conn.execute(
                f"DELETE FROM vip_gift_tokens WHERE token_hash IN ({placeholders})",
                old_hashes,
            ).rowcount
            failure_deleted = conn.execute(
                f"DELETE FROM vip_gift_token_failures WHERE token_hash IN ({placeholders})",
                old_hashes,
            ).rowcount
            return {
                "deletedTokens": max(token_deleted, 0),
                "deletedFailures": max(failure_deleted, 0),
            }

    def delete_test_data(self) -> dict[str, int]:
        with self.connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            claim_deleted = conn.execute(
                """
                DELETE FROM vip_gift_claims
                WHERE receiver_uid LIKE 'TEST_%'
                   OR donor_uid LIKE 'TEST_%'
                   OR record_id LIKE 'TEST_%'
                   OR reward_name LIKE 'TEST_%'
                """
            ).rowcount
            token_hashes = [
                row["token_hash"]
                for row in conn.execute(
                    """
                    SELECT token_hash
                    FROM vip_gift_tokens
                    WHERE token LIKE 'TEST_%'
                       OR donor_uid LIKE 'TEST_%'
                       OR source = 'codex-connectivity-test'
                    """
                )
            ]
            token_deleted = 0
            failure_deleted = 0
            if token_hashes:
                placeholders = ",".join("?" for _ in token_hashes)
                failure_deleted = conn.execute(
                    f"DELETE FROM vip_gift_token_failures WHERE token_hash IN ({placeholders})",
                    token_hashes,
                ).rowcount
                token_deleted = conn.execute(
                    f"DELETE FROM vip_gift_tokens WHERE token_hash IN ({placeholders})",
                    token_hashes,
                ).rowcount
            return {
                "deletedClaims": max(claim_deleted, 0),
                "deletedTokens": max(token_deleted, 0),
                "deletedFailures": max(failure_deleted, 0),
            }

    def stats(self) -> dict[str, Any]:
        with self.connect() as conn:
            token_rows = conn.execute(
                """
                SELECT status, COUNT(*) AS count, COALESCE(SUM(available_days), 0) AS days
                FROM vip_gift_tokens
                GROUP BY status
                ORDER BY status
                """
            ).fetchall()
            claim_rows = conn.execute(
                """
                SELECT month, COUNT(*) AS count, COALESCE(SUM(duration), 0) AS days
                FROM vip_gift_claims
                GROUP BY month
                ORDER BY month DESC
                LIMIT 12
                """
            ).fetchall()
            return {
                "tokens": [dict(row) for row in token_rows],
                "claimsByMonth": [dict(row) for row in claim_rows],
            }

    def clear_failures(self, receiver_uid: str = "") -> int:
        receiver_key = receiver_hash(receiver_uid) if receiver_uid else ""
        with self.connect() as conn:
            if receiver_key:
                return conn.execute(
                    "DELETE FROM vip_gift_token_failures WHERE receiver_uid = ?",
                    (receiver_key,),
                ).rowcount
            else:
                return conn.execute("DELETE FROM vip_gift_token_failures").rowcount

    def claim_status(self, month: str, receiver_uid: str) -> dict[str, Any]:
        receiver_key = receiver_hash(receiver_uid)
        with self.connect() as conn:
            row = conn.execute(
                """
                SELECT *
                FROM vip_gift_claims
                WHERE month = ? AND receiver_uid = ?
                """,
                (month, receiver_key),
            ).fetchone()
            return {"claimed": row is not None, "claim": row_to_dict(row)}

    def upsert_token(self, body: dict[str, Any]) -> dict[str, Any]:
        token = as_text(body.get("token"))
        if not token:
            raise ValueError("token is required")
        thash = token_hash(token)
        month = parse_month(body.get("month"))
        donor_uid = as_text(body.get("donorUid") or body.get("donor_uid"))
        vip_level = as_text(body.get("vipLevel") or body.get("vip_level"))
        total_days = parse_int(
            body.get("totalDays")
            or body.get("total_days")
            or body.get("inviterTotalDays")
            or body.get("inviter_total_days"),
            0,
        )
        if total_days <= 0:
            total_days = vip_level_to_days(vip_level)
        available_days = parse_int(body.get("availableDays") or body.get("available_days"), total_days)
        expire_time_ms = normalize_millis(
            body.get("expireTime")
            or body.get("expire_time")
            or body.get("expireTimeMs")
            or body.get("expire_time_ms")
            or body.get("tokenExpireTime")
            or body.get("token_expire_time")
        )
        source = as_text(body.get("source"))
        metadata = body.get("metadata") or {}
        if not isinstance(metadata, dict):
            metadata = {"value": metadata}
        metadata_json = json.dumps(metadata, ensure_ascii=False, sort_keys=True)
        ts = now_ms()

        with self.connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            existing = conn.execute(
                "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                (thash,),
            ).fetchone()
            if existing is None:
                status = "available"
                if expire_time_ms > 0 and expire_time_ms <= ts:
                    status = "expired"
                elif available_days < self.min_available_days:
                    status = "depleted"
                conn.execute(
                    """
                    INSERT INTO vip_gift_tokens (
                        token_hash, token, donor_uid, month, vip_level, total_days,
                        available_days, claimed_days, claim_count, expire_time_ms,
                        status, source, metadata_json, created_at_ms, updated_at_ms
                    ) VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        thash,
                        token,
                        donor_uid,
                        month,
                        vip_level,
                        max(total_days, 0),
                        max(available_days, 0),
                        expire_time_ms,
                        status,
                        source,
                        metadata_json,
                        ts,
                        ts,
                    ),
                )
            else:
                new_total = max(parse_int(existing["total_days"], 0), total_days)
                current_available = parse_int(existing["available_days"], 0)
                if current_available <= 0 and available_days > 0 and parse_int(existing["claim_count"], 0) == 0:
                    current_available = available_days
                status = existing["status"]
                claim_count = parse_int(existing["claim_count"], 0)
                if status == "depleted" and claim_count == 0 and current_available >= self.min_available_days:
                    status = "available"
                elif status == "expired" and expire_time_ms > ts and current_available >= self.min_available_days:
                    status = "available"
                elif status not in TERMINAL_TOKEN_STATUSES:
                    if expire_time_ms > 0 and expire_time_ms <= ts:
                        status = "expired"
                    elif current_available < self.min_available_days:
                        status = "depleted"
                    else:
                        status = "available"
                conn.execute(
                    """
                    UPDATE vip_gift_tokens
                    SET token = ?,
                        donor_uid = COALESCE(NULLIF(?, ''), donor_uid),
                        month = ?,
                        vip_level = COALESCE(NULLIF(?, ''), vip_level),
                        total_days = ?,
                        available_days = ?,
                        expire_time_ms = CASE WHEN ? > 0 THEN ? ELSE expire_time_ms END,
                        status = ?,
                        source = COALESCE(NULLIF(?, ''), source),
                        metadata_json = ?,
                        updated_at_ms = ?
                    WHERE token_hash = ?
                    """,
                    (
                        token,
                        donor_uid,
                        month,
                        vip_level,
                        new_total,
                        current_available,
                        expire_time_ms,
                        expire_time_ms,
                        status,
                        source,
                        metadata_json,
                        ts,
                        thash,
                    ),
                )
            row = conn.execute(
                "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                (thash,),
            ).fetchone()
            return {"tokenHash": thash, "token": public_token_row(row, include_token=False)}

    def available_token(self, month: str, receiver_uid: str, exclude_donor_uid: str = "") -> dict[str, Any]:
        self.prune(month=month)
        receiver_key = receiver_hash(receiver_uid) if receiver_uid else ""
        with self.connect() as conn:
            if receiver_key:
                claim = conn.execute(
                    """
                    SELECT *
                    FROM vip_gift_claims
                    WHERE month = ? AND receiver_uid = ?
                    """,
                    (month, receiver_key),
                ).fetchone()
                if claim is not None:
                    return {"claimed": True, "claim": row_to_dict(claim), "token": None}

            params: list[Any] = [month, self.min_available_days, now_ms()]
            failure_filter = ""
            if receiver_key:
                failure_filter = """
                    AND NOT EXISTS (
                        SELECT 1
                        FROM vip_gift_token_failures f
                        WHERE f.month = t.month
                          AND f.receiver_uid = ?
                          AND f.token_hash = t.token_hash
                          AND f.created_at_ms > ?
                    )
                """
                params.append(receiver_key)
                params.append(now_ms() - self.failure_expire_ms)
            donor_filter = ""
            if exclude_donor_uid:
                donor_filter = "AND t.donor_uid <> ?"
                params.append(exclude_donor_uid)

            row = conn.execute(
                f"""
                SELECT t.*
                FROM vip_gift_tokens t
                WHERE t.month = ?
                  AND t.status = 'available'
                  AND t.available_days >= ?
                  AND (t.expire_time_ms = 0 OR t.expire_time_ms > ?)
                  {failure_filter}
                  {donor_filter}
                ORDER BY t.available_days DESC, t.updated_at_ms ASC
                LIMIT 1
                """,
                params,
            ).fetchone()
            return {"claimed": False, "token": available_token_row(row)}

    def record_claim_success(self, body: dict[str, Any]) -> dict[str, Any]:
        month = parse_month(body.get("month"))
        receiver_uid = as_text(body.get("receiverUid") or body.get("receiver_uid"))
        if not receiver_uid:
            raise ValueError("receiverUid is required")
        receiver_key = receiver_hash(receiver_uid)
        token = as_text(body.get("token"))
        thash = as_text(body.get("tokenHash") or body.get("token_hash"))
        if not thash and token:
            thash = token_hash(token)
        if not thash:
            raise ValueError("tokenHash or token is required")

        duration = parse_int(body.get("duration"), 0)
        if duration <= 0:
            raise ValueError("duration from accept response is required")
        record_id = as_text(body.get("recordId") or body.get("record_id"))
        reward_name = as_text(body.get("rewardName") or body.get("reward_name"))
        accepted_at_ms = normalize_millis(body.get("acceptedAt") or body.get("accepted_at") or now_ms())
        ts = now_ms()

        with self.connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            existing_claim = conn.execute(
                """
                SELECT *
                FROM vip_gift_claims
                WHERE month = ? AND receiver_uid = ?
                """,
                (month, receiver_key),
            ).fetchone()
            if existing_claim is not None:
                token_row = conn.execute(
                    "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                    (existing_claim["token_hash"],),
                ).fetchone()
                return {
                    "duplicate": True,
                    "claim": row_to_dict(existing_claim),
                    "token": public_token_row(token_row, include_token=False),
                }

            token_row = conn.execute(
                "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                (thash,),
            ).fetchone()
            if token_row is None:
                raise ValueError("tokenHash is not registered")

            available_days = max(parse_int(token_row["available_days"], 0) - duration, 0)
            claimed_days = parse_int(token_row["claimed_days"], 0) + duration
            claim_count = parse_int(token_row["claim_count"], 0) + 1
            status = "available" if available_days >= self.min_available_days else "depleted"

            conn.execute(
                """
                INSERT INTO vip_gift_claims (
                    month, receiver_uid, token_hash, donor_uid, record_id, duration,
                    reward_name, accepted_at_ms, created_at_ms, updated_at_ms
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    month,
                    receiver_key,
                    thash,
                    token_row["donor_uid"],
                    record_id,
                    duration,
                    reward_name,
                    accepted_at_ms,
                    ts,
                    ts,
                ),
            )
            conn.execute(
                """
                UPDATE vip_gift_tokens
                SET available_days = ?,
                    claimed_days = ?,
                    claim_count = ?,
                    status = ?,
                    updated_at_ms = ?
                WHERE token_hash = ?
                """,
                (available_days, claimed_days, claim_count, status, ts, thash),
            )
            updated = conn.execute(
                "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                (thash,),
            ).fetchone()
            claim = conn.execute(
                """
                SELECT *
                FROM vip_gift_claims
                WHERE month = ? AND receiver_uid = ?
                """,
                (month, receiver_key),
            ).fetchone()
            return {
                "duplicate": False,
                "claim": row_to_dict(claim),
                "token": public_token_row(updated, include_token=False),
            }

    def record_token_fail(self, body: dict[str, Any]) -> dict[str, Any]:
        month = parse_month(body.get("month"))
        receiver_uid = as_text(body.get("receiverUid") or body.get("receiver_uid"))
        receiver_key = receiver_hash(receiver_uid) if receiver_uid else ""
        token = as_text(body.get("token"))
        thash = as_text(body.get("tokenHash") or body.get("token_hash"))
        if not thash and token:
            thash = token_hash(token)
        if not thash:
            raise ValueError("tokenHash or token is required")

        reason = as_text(body.get("reason") or body.get("status") or "failed").lower()
        message = as_text(body.get("message"))
        available_days = body.get("availableDays") or body.get("available_days")
        ts = now_ms()

        with self.connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            conn.execute(
                """
                INSERT OR IGNORE INTO vip_gift_token_failures (
                    month, receiver_uid, token_hash, reason, message, created_at_ms
                ) VALUES (?, ?, ?, ?, ?, ?)
                """,
                (month, receiver_key, thash, reason, message, ts),
            )

            status_updated = False
            if reason in TERMINAL_TOKEN_STATUSES:
                conn.execute(
                    """
                    UPDATE vip_gift_tokens
                    SET status = ?, updated_at_ms = ?
                    WHERE token_hash = ?
                    """,
                    (reason, ts, thash),
                )
                status_updated = True
            elif available_days is not None:
                days = parse_int(available_days, 0)
                status = "available" if days >= self.min_available_days else "depleted"
                conn.execute(
                    """
                    UPDATE vip_gift_tokens
                    SET available_days = ?, status = ?, updated_at_ms = ?
                    WHERE token_hash = ?
                    """,
                    (days, status, ts, thash),
                )
                status_updated = True

            row = conn.execute(
                "SELECT * FROM vip_gift_tokens WHERE token_hash = ?",
                (thash,),
            ).fetchone()
            return {
                "statusUpdated": status_updated,
                "token": public_token_row(row, include_token=False),
            }


class ApiHandler(BaseHTTPRequestHandler):
    server_version = f"{APP_NAME}/1.0"

    @property
    def store(self) -> VipGiftStore:
        return self.server.store  # type: ignore[attr-defined]

    @property
    def api_token(self) -> str:
        return self.server.api_token  # type: ignore[attr-defined]

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("%s - - [%s] %s\n" % (self.address_string(), self.log_date_time_string(), fmt % args))

    def do_GET(self) -> None:
        self.dispatch()

    def do_POST(self) -> None:
        self.dispatch()

    def dispatch(self) -> None:
        try:
            parsed = urlparse(self.path)
            if not self.authorized(parsed.path):
                self.write_json({"ok": False, "error": "unauthorized"}, status=HTTPStatus.UNAUTHORIZED)
                return

            if self.command == "GET" and parsed.path == "/health":
                self.write_json(
                    {
                        "ok": True,
                        "app": APP_NAME,
                        "schemaVersion": SCHEMA_VERSION,
                        "month": current_month(),
                        "nowMs": now_ms(),
                    }
                )
                return

            if self.command == "GET" and parsed.path == "/stats":
                self.write_json({"ok": True, "data": self.store.stats()})
                return

            if (self.command == "GET" or self.command == "POST") and parsed.path == "/claims/failures/clear":
                query = parse_qs(parsed.query)
                receiver_uid = as_text(first(query, "receiverUid") or first(query, "receiver_uid"))
                deleted = self.store.clear_failures(receiver_uid)
                self.write_json({"ok": True, "deleted": deleted})
                return

            if self.command == "GET" and parsed.path == "/claims/status":
                query = parse_qs(parsed.query)
                month = parse_month(first(query, "month"))
                receiver_uid = as_text(first(query, "receiverUid") or first(query, "receiver_uid"))
                if not receiver_uid:
                    self.write_json({"ok": False, "error": "receiverUid is required"}, HTTPStatus.BAD_REQUEST)
                    return
                self.write_json({"ok": True, "data": self.store.claim_status(month, receiver_uid)})
                return

            if self.command == "GET" and parsed.path == "/tokens/available":
                query = parse_qs(parsed.query)
                month = parse_month(first(query, "month"))
                receiver_uid = as_text(first(query, "receiverUid") or first(query, "receiver_uid"))
                exclude_donor_uid = as_text(first(query, "excludeDonorUid") or first(query, "exclude_donor_uid"))
                self.write_json({"ok": True, "data": self.store.available_token(month, receiver_uid, exclude_donor_uid)})
                return

            if self.command == "POST" and parsed.path == "/tokens/upsert":
                self.write_json({"ok": True, "data": self.store.upsert_token(self.read_body())})
                return

            if self.command == "POST" and parsed.path == "/claims/success":
                self.write_json({"ok": True, "data": self.store.record_claim_success(self.read_body())})
                return

            if self.command == "POST" and parsed.path == "/tokens/fail":
                self.write_json({"ok": True, "data": self.store.record_token_fail(self.read_body())})
                return

            if self.command == "POST" and parsed.path == "/maintenance/prune":
                body = self.read_body(optional=True)
                month = body.get("month") if isinstance(body, dict) else None
                min_days = body.get("minAvailableDays") if isinstance(body, dict) else None
                result = self.store.prune(month=month, min_available_days=parse_int(min_days, self.store.min_available_days))
                self.write_json({"ok": True, "data": result})
                return

            if self.command == "POST" and parsed.path == "/maintenance/delete-test-data":
                self.write_json({"ok": True, "data": self.store.delete_test_data()})
                return

            self.write_json({"ok": False, "error": "not found"}, status=HTTPStatus.NOT_FOUND)
        except ValueError as exc:
            self.write_json({"ok": False, "error": str(exc)}, status=HTTPStatus.BAD_REQUEST)
        except Exception as exc:  # pragma: no cover - final safety net for server mode.
            traceback.print_exc()
            self.write_json({"ok": False, "error": str(exc)}, status=HTTPStatus.INTERNAL_SERVER_ERROR)

    def authorized(self, path: str) -> bool:
        if path == "/health":
            return True
        if not self.api_token:
            return True
        auth = self.headers.get("Authorization", "")
        api_key = self.headers.get("X-Api-Key", "")
        if auth.lower().startswith("bearer "):
            candidate = auth[7:].strip()
        else:
            candidate = api_key.strip()
        return hmac.compare_digest(candidate, self.api_token)

    def read_body(self, optional: bool = False) -> dict[str, Any]:
        length = parse_int(self.headers.get("Content-Length"), 0)
        if length <= 0:
            if optional:
                return {}
            raise ValueError("request body is required")
        if length > MAX_BODY_BYTES:
            raise ValueError("request body is too large")
        raw = self.rfile.read(length)
        if not raw.strip() and optional:
            return {}
        try:
            data = json.loads(raw.decode("utf-8"))
        except json.JSONDecodeError as exc:
            raise ValueError(f"invalid json body: {exc}") from exc
        if not isinstance(data, dict):
            raise ValueError("json body must be an object")
        return data

    def write_json(self, data: Any, status: HTTPStatus = HTTPStatus.OK) -> None:
        body = json_dumps(data)
        self.send_response(int(status))
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)


def first(query: dict[str, list[str]], key: str) -> str:
    values = query.get(key) or []
    return values[0] if values else ""


class VipGiftHTTPServer(ThreadingHTTPServer):
    def __init__(self, server_address: tuple[str, int], handler_class: type[BaseHTTPRequestHandler], store: VipGiftStore, api_token: str) -> None:
        super().__init__(server_address, handler_class)
        self.store = store
        self.api_token = api_token


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="VIP member gift cloud coordinator")
    parser.add_argument("--host", default=os.getenv("VIP_GIFT_HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=parse_int(os.getenv("VIP_GIFT_PORT"), 3102))
    parser.add_argument("--db", default=os.getenv("VIP_GIFT_DB_PATH", "vip_member_gift_cloud.sqlite3"))
    parser.add_argument(
        "--token",
        default=os.getenv("VIP_GIFT_CLOUD_TOKEN", ""),
        help="optional API token; also configurable via VIP_GIFT_CLOUD_TOKEN",
    )
    parser.add_argument(
        "--min-available-days",
        type=int,
        default=parse_int(os.getenv("VIP_GIFT_MIN_AVAILABLE_DAYS"), DEFAULT_MIN_AVAILABLE_DAYS),
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_arg_parser().parse_args(argv)
    store = VipGiftStore(args.db, max(args.min_available_days, 1))
    prune_result = store.prune(min_available_days=store.min_available_days)
    server = VipGiftHTTPServer((args.host, args.port), ApiHandler, store, args.token)
    print(
        f"{APP_NAME} listening on http://{args.host}:{args.port} "
        f"db={args.db} minAvailableDays={store.min_available_days} "
        f"auth={'on' if args.token else 'off'} pruned={prune_result}",
        flush=True,
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nshutting down", flush=True)
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
