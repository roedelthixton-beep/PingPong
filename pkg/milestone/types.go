package milestone

import "errors"

const (
	MetricConsumed = "consumed"
	MetricScore1   = "score_1"
	MetricScore2   = "score_2"

	NotificationTypeMilestone = "milestone"
)

var (
	ErrInvalidMetricKey = errors.New("invalid milestone metric key")
	ErrNilDB            = errors.New("milestone db is nil")
	ErrNilRedisClient   = errors.New("milestone redis client is nil")
	ErrNilIDGenerator   = errors.New("milestone id generator is nil")
)

type Notification struct {
	NotificationID string `json:"notification_id"`
	Type           string `json:"type"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"created_at"`
}

type TemplateData struct {
	ItemID       int64
	Threshold    int64
	CounterValue int64
	ItemSummary  string
}

type IDGenerator interface {
	NextID() (int64, error)
}

func IsValidMetricKey(metricKey string) bool {
	switch metricKey {
	case MetricConsumed, MetricScore1, MetricScore2:
		return true
	default:
		return false
	}
}

func isValidMetricKey(metricKey string) bool {
	return IsValidMetricKey(metricKey)
}
