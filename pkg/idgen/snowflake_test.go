package idgen

import "testing"

func TestNewSnowflakeWorkerRange(t *testing.T) {
	if _, err := NewSnowflake(-1, 1704067200000); err == nil {
		t.Fatalf("expected error for negative worker id")
	}
	if _, err := NewSnowflake(maxWorkerID+1, 1704067200000); err == nil {
		t.Fatalf("expected error for too large worker id")
	}
}

func TestSnowflakeNextIDMonotonic(t *testing.T) {
	sf, err := NewSnowflake(7, 1704067200000)
	if err != nil {
		t.Fatalf("NewSnowflake error: %v", err)
	}

	var now int64 = 1704067201000
	sf.nowMillis = func() int64 {
		return now
	}

	prev := int64(0)
	for i := 0; i < 100; i++ {
		id, err := sf.NextID()
		if err != nil {
			t.Fatalf("NextID error: %v", err)
		}
		if id <= prev {
			t.Fatalf("ids must be increasing: prev=%d current=%d", prev, id)
		}
		prev = id
	}
}

func TestSnowflakeClockRollbackWaits(t *testing.T) {
	sf, err := NewSnowflake(1, 1704067200000)
	if err != nil {
		t.Fatalf("NewSnowflake error: %v", err)
	}

	timeline := []int64{
		1704067202000, // first call
		1704067201999, // rollback detected
		1704067202000, // waitUntil catches up
	}
	idx := 0
	sf.nowMillis = func() int64 {
		if idx >= len(timeline) {
			return timeline[len(timeline)-1]
		}
		v := timeline[idx]
		idx++
		return v
	}

	first, err := sf.NextID()
	if err != nil {
		t.Fatalf("first NextID error: %v", err)
	}
	second, err := sf.NextID()
	if err != nil {
		t.Fatalf("second NextID error: %v", err)
	}
	if second <= first {
		t.Fatalf("expected second id > first id, got first=%d second=%d", first, second)
	}
}
