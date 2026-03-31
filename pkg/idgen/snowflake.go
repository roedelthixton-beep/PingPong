package idgen

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	workerIDBits  = 10
	sequenceBits  = 12
	maxWorkerID   = -1 ^ (-1 << workerIDBits)
	sequenceMask  = -1 ^ (-1 << sequenceBits)
	workerIDShift = sequenceBits
	timeShift     = sequenceBits + workerIDBits
)

// Snowflake is a 64-bit ID generator:
// 1 sign bit (unused) + 41 timestamp bits + 10 worker bits + 12 sequence bits.
type Snowflake struct {
	mu sync.Mutex

	epochMS   int64
	workerID  int64
	lastMS    int64
	sequence  int64
	nowMillis func() int64
}

func NewSnowflake(workerID int64, epochMS int64) (*Snowflake, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, fmt.Errorf("worker_id out of range: %d", workerID)
	}
	if epochMS <= 0 {
		return nil, errors.New("invalid epoch")
	}

	return &Snowflake{
		epochMS:   epochMS,
		workerID:  workerID,
		nowMillis: func() int64 { return time.Now().UnixMilli() },
	}, nil
}

func (s *Snowflake) WorkerID() int64 {
	return s.workerID
}

func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowMillis()
	if now < s.lastMS {
		// If clock moves backwards, wait until last observed timestamp.
		now = s.waitUntil(s.lastMS)
	}

	if now == s.lastMS {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			now = s.waitUntil(s.lastMS + 1)
		}
	} else {
		s.sequence = 0
	}

	if now < s.epochMS {
		return 0, errors.New("current time is before epoch")
	}

	s.lastMS = now
	id := ((now - s.epochMS) << timeShift) | (s.workerID << workerIDShift) | s.sequence
	if id <= 0 {
		return 0, errors.New("generated invalid id")
	}
	return id, nil
}

func (s *Snowflake) waitUntil(targetMS int64) int64 {
	now := s.nowMillis()
	for now < targetMS {
		time.Sleep(time.Millisecond)
		now = s.nowMillis()
	}
	return now
}
