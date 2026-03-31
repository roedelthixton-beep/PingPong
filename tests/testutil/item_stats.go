package testutil

import (
	"testing"
	"time"
)

type ItemStatsSnapshot struct {
	ConsumedCount int64
	Score1Count   int64
	Score2Count   int64
	TotalScore    int64
}

func WaitForItemStats(t *testing.T, itemID int64, timeout time.Duration, predicate func(ItemStatsSnapshot) bool) ItemStatsSnapshot {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var snapshot ItemStatsSnapshot
		err := TestDB.QueryRow(
			"SELECT consumed_count, score_1_count, score_2_count, total_score FROM item_stats WHERE item_id = $1",
			itemID,
		).Scan(&snapshot.ConsumedCount, &snapshot.Score1Count, &snapshot.Score2Count, &snapshot.TotalScore)
		if err == nil && predicate(snapshot) {
			return snapshot
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for item_stats for item %d", itemID)
	return ItemStatsSnapshot{}
}
