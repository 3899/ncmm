#!/usr/bin/env python3
"""Unit and HTTP integration tests for vip_member_gift_cloud."""

from __future__ import annotations

import contextlib
import json
import os
import sqlite3
import tempfile
import threading
import unittest
import urllib.error
import urllib.request

import vip_member_gift_cloud as cloud


class VipMemberGiftCloudTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmpdir = tempfile.TemporaryDirectory()
        self.db_path = os.path.join(self.tmpdir.name, "test.sqlite3")

    def tearDown(self) -> None:
        self.tmpdir.cleanup()

    def _seed_tokens(self, store: cloud.VipGiftStore, month: str = "2026-11") -> None:
        store.upsert_token(
            {
                "token": "tok_normal_high",
                "donorUid": "normal_1",
                "month": month,
                "availableDays": 200,
            }
        )
        store.upsert_token(
            {
                "token": "tok_normal_low",
                "donorUid": "normal_2",
                "month": month,
                "availableDays": 50,
            }
        )
        store.upsert_token(
            {
                "token": "tok_p2",
                "donorUid": "priority_2",
                "month": month,
                "availableDays": 120,
            }
        )
        store.upsert_token(
            {
                "token": "tok_p1",
                "donorUid": "priority_1",
                "month": month,
                "availableDays": 30,
            }
        )

    def _record_prior_claim(
        self,
        store: cloud.VipGiftStore,
        receiver_uid: str,
        month: str = "2026-10",
        donor_uid: str = "old_normal_donor",
    ) -> None:
        tok = f"tok_{month}_{donor_uid}"
        store.upsert_token(
            {
                "token": tok,
                "donorUid": donor_uid,
                "month": month,
                "availableDays": 100,
            }
        )
        store.record_claim_success(
            {
                "month": month,
                "receiverUid": receiver_uid,
                "token": tok,
                "duration": 7,
            }
        )

    def test_first_time_fallback_default_behavior(self) -> None:
        store = cloud.VipGiftStore(
            self.db_path,
            min_available_days=7,
            priority_donors="priority_1,priority_2",
        )
        self.assertEqual(store.priority_mode, "first_time_fallback")
        self._seed_tokens(store, month="2026-11")
        self._record_prior_claim(store, "uid_old", month="2026-09", donor_uid="other_donor")

        # Never-claimed UID gets priority_1 first, even though normal_1 has more days.
        new_res = store.available_token("2026-11", "uid_new")
        self.assertFalse(new_res["claimed"])
        self.assertEqual(new_res["token"]["token"], "tok_p1")

        # Returning UID gets normal pool first (highest availableDays).
        old_res = store.available_token("2026-11", "uid_old")
        self.assertFalse(old_res["claimed"])
        self.assertEqual(old_res["token"]["token"], "tok_normal_high")

        # Exhaust normal pool for uid_old via failures -> falls back to priority pool in order.
        store.record_token_fail(
            {"month": "2026-11", "receiverUid": "uid_old", "token": "tok_normal_high", "reason": "failed"}
        )
        old_res_2 = store.available_token("2026-11", "uid_old")
        self.assertEqual(old_res_2["token"]["token"], "tok_normal_low")

        store.record_token_fail(
            {"month": "2026-11", "receiverUid": "uid_old", "token": "tok_normal_low", "reason": "failed"}
        )
        old_fallback = store.available_token("2026-11", "uid_old")
        self.assertEqual(old_fallback["token"]["token"], "tok_p1")

        # Anonymous request (empty receiverUid) does not take first-time priority slots.
        anon_res = store.available_token("2026-11", "")
        self.assertEqual(anon_res["token"]["token"], "tok_normal_high")

    def test_always_mode_prioritizes_all_receivers(self) -> None:
        store = cloud.VipGiftStore(
            self.db_path,
            min_available_days=7,
            priority_donors="priority_1,priority_2",
            priority_mode="always",
        )
        self._seed_tokens(store, month="2026-11")
        self._record_prior_claim(store, "uid_old", month="2026-10", donor_uid="other_donor")

        res = store.available_token("2026-11", "uid_old")
        self.assertEqual(res["token"]["token"], "tok_p1")

    def test_first_time_only_mode_excludes_priority_for_returning_users(self) -> None:
        store = cloud.VipGiftStore(
            self.db_path,
            min_available_days=7,
            priority_donors="priority_1,priority_2",
            priority_mode="first_time_only",
        )
        self._seed_tokens(store, month="2026-11")
        self._record_prior_claim(store, "uid_old", month="2026-10", donor_uid="other_donor")

        new_res = store.available_token("2026-11", "uid_new")
        self.assertEqual(new_res["token"]["token"], "tok_p1")

        old_res = store.available_token("2026-11", "uid_old")
        self.assertEqual(old_res["token"]["token"], "tok_normal_high")

        # Mark all normal tokens invalid -> returning user gets None (no fallback to priority).
        store.record_token_fail({"month": "2026-11", "token": "tok_normal_high", "reason": "invalid"})
        store.record_token_fail({"month": "2026-11", "token": "tok_normal_low", "reason": "invalid"})
        old_empty = store.available_token("2026-11", "uid_old")
        self.assertIsNone(old_empty["token"])

        # New user still receives priority_1.
        new_still = store.available_token("2026-11", "uid_new")
        self.assertEqual(new_still["token"]["token"], "tok_p1")

    def test_invalid_priority_mode_raises(self) -> None:
        with self.assertRaises(ValueError):
            cloud.VipGiftStore(self.db_path, min_available_days=7, priority_mode="bad_mode")

    def test_existing_db_upgrade_adds_index_and_uses_index_plan(self) -> None:
        # Simulate a legacy database created before idx_claims_receiver_uid existed.
        with contextlib.closing(sqlite3.connect(self.db_path)) as conn, conn:
            conn.execute(
                """
                CREATE TABLE vip_gift_claims (
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
                )
                """
            )
            conn.execute(
                """
                INSERT INTO vip_gift_claims (
                    month, receiver_uid, token_hash, donor_uid, created_at_ms, updated_at_ms
                ) VALUES ('2026-07', ?, 'legacy_hash', 'legacy_donor', 1000, 1000)
                """,
                (cloud.receiver_hash("legacy_uid"),),
            )

        store = cloud.VipGiftStore(
            self.db_path,
            min_available_days=7,
            priority_donors="priority_1",
        )
        self._seed_tokens(store, month="2026-11")

        # Legacy UID from 2026-07 is recognized as returning user.
        res = store.available_token("2026-11", "legacy_uid")
        self.assertEqual(res["token"]["token"], "tok_normal_high")

        # Verify SQLite uses idx_claims_receiver_uid for cross-month lookup.
        with store.connect() as conn:
            plan_rows = conn.execute(
                "EXPLAIN QUERY PLAN SELECT 1 FROM vip_gift_claims WHERE receiver_uid = ? LIMIT 1",
                (cloud.receiver_hash("legacy_uid"),),
            ).fetchall()
        plan_text = " ".join(str(r[3]) for r in plan_rows)
        self.assertIn("idx_claims_receiver_uid", plan_text)

    def test_http_health_and_query_override(self) -> None:
        store = cloud.VipGiftStore(
            self.db_path,
            min_available_days=7,
            priority_donors="priority_1",
            priority_mode="first_time_fallback",
        )
        self._seed_tokens(store, month="2026-11")
        self._record_prior_claim(store, "uid_old", month="2026-10", donor_uid="other_donor")

        server = cloud.VipGiftHTTPServer(("127.0.0.1", 0), cloud.ApiHandler, store, api_token="")
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/health") as resp:
                health = json.loads(resp.read().decode("utf-8"))
            self.assertEqual(health["priorityMode"], "first_time_fallback")
            self.assertEqual(health["priorityDonors"], ["priority_1"])

            with urllib.request.urlopen(
                f"http://127.0.0.1:{port}/tokens/available?month=2026-11&receiverUid=uid_old"
            ) as resp:
                default_data = json.loads(resp.read().decode("utf-8"))["data"]
            self.assertEqual(default_data["token"]["token"], "tok_normal_high")

            with urllib.request.urlopen(
                f"http://127.0.0.1:{port}/tokens/available?month=2026-11&receiverUid=uid_old&priorityMode=always"
            ) as resp:
                override_data = json.loads(resp.read().decode("utf-8"))["data"]
            self.assertEqual(override_data["token"]["token"], "tok_p1")

            with self.assertRaises(urllib.error.HTTPError) as ctx:
                urllib.request.urlopen(
                    f"http://127.0.0.1:{port}/tokens/available?month=2026-11&receiverUid=uid_old&priorityMode=invalid"
                )
            self.assertEqual(ctx.exception.code, 400)
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)


if __name__ == "__main__":
    unittest.main()
