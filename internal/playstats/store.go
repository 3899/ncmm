// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package playstats

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/3899/ncmm/pkg/database"
)

const dateLayout = "2006-01-02"

const OfficialDailyReadyHour = 9

var ChinaLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type Point struct {
	Date        string    `json:"date"`
	Count       int       `json:"count"`
	Source      string    `json:"source,omitempty"`
	CollectedAt time.Time `json:"collectedAt,omitempty"`
}

type DailyPoint struct {
	Date        string     `json:"date"`
	Count       int        `json:"count"`
	Known       bool       `json:"known"`
	Source      string     `json:"source,omitempty"`
	CollectedAt *time.Time `json:"collectedAt,omitempty"`
}

type AccountSeries struct {
	Account string       `json:"account"`
	Points  []DailyPoint `json:"points"`
}

func CanonicalAccount(path, baseDir string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "${HOME}", baseDir))
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return filepath.ToSlash(path)
}

func RecentCompleteDates(now time.Time, count int) []string {
	if count <= 0 {
		return nil
	}
	chinaNow := now.In(ChinaLocation)
	latest := chinaNow.AddDate(0, 0, -1)
	if chinaNow.Hour() < OfficialDailyReadyHour {
		latest = latest.AddDate(0, 0, -1)
	}
	dates := make([]string, count)
	for index := range count {
		dates[index] = latest.AddDate(0, 0, index-count+1).Format(dateLayout)
	}
	return dates
}

// MarkAttemptIfNew records a request attempt before it is sent. This prevents
// repeated task executions from amplifying upstream requests after a failure.
func MarkAttemptIfNew(ctx context.Context, db database.Database, account, operation, date string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is unavailable")
	}
	key := attemptKey(account, operation, date)
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := db.Set(ctx, key, time.Now().In(ChinaLocation).Format(time.RFC3339), 45*24*time.Hour); err != nil {
		return false, err
	}
	return true, nil
}

func ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, value, ChinaLocation)
}

func Save(ctx context.Context, db database.Database, account string, point Point) error {
	if db == nil {
		return fmt.Errorf("database is unavailable")
	}
	if _, err := ParseDate(point.Date); err != nil {
		return fmt.Errorf("invalid play-stat date %q: %w", point.Date, err)
	}
	if point.Count < 0 {
		return fmt.Errorf("play-stat count cannot be negative")
	}
	data, err := json.Marshal(point)
	if err != nil {
		return fmt.Errorf("marshal play stat: %w", err)
	}
	return db.Set(ctx, key(account, point.Date), string(data))
}

func Load(ctx context.Context, db database.Database, account, date string) (Point, bool, error) {
	if db == nil {
		return Point{}, false, fmt.Errorf("database is unavailable")
	}
	value, err := db.Get(ctx, key(account, date))
	if err != nil {
		exists, existsErr := db.Exists(ctx, key(account, date))
		if existsErr == nil && !exists {
			return Point{}, false, nil
		}
		return Point{}, false, err
	}
	var point Point
	if err := json.Unmarshal([]byte(value), &point); err != nil {
		return Point{}, false, fmt.Errorf("decode play stat %s: %w", date, err)
	}
	return point, true, nil
}

func LoadSeries(ctx context.Context, db database.Database, account string, dates []string) ([]DailyPoint, error) {
	result := make([]DailyPoint, 0, len(dates))
	for _, date := range dates {
		point, known, err := Load(ctx, db, account, date)
		if err != nil {
			return nil, err
		}
		var collectedAt *time.Time
		if known && !point.CollectedAt.IsZero() {
			value := point.CollectedAt
			collectedAt = &value
		}
		result = append(result, DailyPoint{
			Date: date, Count: point.Count, Known: known,
			Source: point.Source, CollectedAt: collectedAt,
		})
	}
	return result, nil
}

func MissingDates(points []DailyPoint) []string {
	var missing []string
	for _, point := range points {
		if !point.Known {
			missing = append(missing, point.Date)
		}
	}
	sort.Strings(missing)
	return missing
}

func key(account, date string) string {
	sum := sha256.Sum256([]byte(account))
	return "musician:play-stats:v1:" + hex.EncodeToString(sum[:]) + ":" + date
}

func attemptKey(account, operation, date string) string {
	sum := sha256.Sum256([]byte(account))
	return "musician:play-stats-attempt:v1:" + hex.EncodeToString(sum[:]) + ":" + operation + ":" + date
}
