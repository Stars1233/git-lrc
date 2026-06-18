package reviewquery

import (
	"fmt"
	"testing"
	"time"
)

func syntheticRecords(n int) []ReviewRecord {
	actions := []string{"reviewed", "vouched", "skipped", "none"}
	recs := make([]ReviewRecord, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		recs[i] = ReviewRecord{
			Hash:        fmt.Sprintf("%040x", i),
			ShortHash:   fmt.Sprintf("%07x", i),
			Author:      fmt.Sprintf("dev%d", i%10),
			Email:       fmt.Sprintf("dev%d@example.com", i%10),
			Date:        base.Add(time.Duration(i) * time.Hour),
			Branch:      "main",
			Subject:     "commit subject",
			Action:      actions[i%len(actions)],
			Iterations:  i % 5,
			CoveragePct: i % 101,
		}
	}
	return recs
}

func TestRunOnRecords(t *testing.T) {
	recs := syntheticRecords(8) // 2 of each action
	res, err := RunOnRecords(recs, "SELECT action, COUNT(*) AS n FROM review_log GROUP BY action ORDER BY action")
	if err != nil {
		t.Fatalf("RunOnRecords error: %v", err)
	}
	if len(res.Columns) != 2 || res.Columns[0] != "action" {
		t.Errorf("unexpected columns: %v", res.Columns)
	}
	if len(res.Rows) != 4 {
		t.Fatalf("expected 4 action groups, got %d: %v", len(res.Rows), res.Rows)
	}
	for _, row := range res.Rows {
		if row[1] != "2" {
			t.Errorf("expected 2 per action, got %v", row)
		}
	}
}

func TestRunOnRecordsEmpty(t *testing.T) {
	res, err := RunOnRecords(nil, "SELECT COUNT(*) AS n FROM review_log")
	if err != nil {
		t.Fatalf("RunOnRecords(nil) error: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "0" {
		t.Errorf("expected count 0 on empty input, got %v", res.Rows)
	}
}

// BenchmarkRunOnRecords measures load+query cost at various repo sizes.
// Run: go test -run=^$ -bench=RunOnRecords -benchmem ./internal/reviewquery/
func BenchmarkRunOnRecords(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		recs := syntheticRecords(n)
		b.Run(fmt.Sprintf("commits=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := RunOnRecords(recs, "SELECT action, COUNT(*) FROM review_log GROUP BY action"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
