package playstats

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memoryDatabase struct {
	values map[string]string
}

func newMemoryDatabase() *memoryDatabase {
	return &memoryDatabase{values: make(map[string]string)}
}

func (db *memoryDatabase) Get(_ context.Context, key string) (string, error) {
	value, ok := db.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (db *memoryDatabase) Set(_ context.Context, key, value string, _ ...time.Duration) error {
	db.values[key] = value
	return nil
}

func (db *memoryDatabase) Exists(_ context.Context, key string) (bool, error) {
	_, ok := db.values[key]
	return ok, nil
}

func (db *memoryDatabase) Increment(_ context.Context, key string, value int64, _ ...time.Duration) (int64, error) {
	old, _ := strconv.ParseInt(db.values[key], 10, 64)
	db.values[key] = strconv.FormatInt(old+value, 10)
	return old, nil
}

func (db *memoryDatabase) Del(_ context.Context, key string) error {
	delete(db.values, key)
	return nil
}

func (db *memoryDatabase) Close(context.Context) error { return nil }

func TestRecentCompleteDates(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, ChinaLocation)
	want := []string{"2026-08-26", "2026-08-27", "2026-08-28", "2026-08-29", "2026-08-30", "2026-08-31", "2026-09-01"}
	if got := RecentCompleteDates(now, 7); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates = %v, want %v", got, want)
	}
}

func TestRecentCompleteDatesBeforeDailyUpdate(t *testing.T) {
	now := time.Date(2026, 9, 2, 8, 59, 0, 0, ChinaLocation)
	want := []string{"2026-08-30", "2026-08-31"}
	if got := RecentCompleteDates(now, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates = %v, want %v", got, want)
	}
}

func TestMarkAttemptIfNew(t *testing.T) {
	db := newMemoryDatabase()
	first, err := MarkAttemptIfNew(t.Context(), db, "account", "daily", "2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarkAttemptIfNew(t.Context(), db, "account", "daily", "2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	otherDay, err := MarkAttemptIfNew(t.Context(), db, "account", "daily", "2026-09-02")
	if err != nil {
		t.Fatal(err)
	}
	if !first || second || !otherDay {
		t.Fatalf("attempt flags = first:%v second:%v otherDay:%v", first, second, otherDay)
	}
}

func TestSaveAndLoadSeriesKeepsUnknownDates(t *testing.T) {
	db := newMemoryDatabase()
	account := CanonicalAccount("cookie.json", t.TempDir())
	if err := Save(t.Context(), db, account, Point{Date: "2026-09-01", Count: 265, Source: "songs/daily"}); err != nil {
		t.Fatal(err)
	}
	series, err := LoadSeries(t.Context(), db, account, []string{"2026-08-31", "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 || series[0].Known || !series[1].Known || series[1].Count != 265 {
		t.Fatalf("unexpected series: %+v", series)
	}
	if !strings.HasSuffix(account, "/cookie.json") {
		t.Fatalf("canonical account = %q", account)
	}
}
