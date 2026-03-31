package dal

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	NotificationStatusPending  int16 = 0
	NotificationStatusNotified int16 = 1
)

type MilestoneRule struct {
	RuleID          int64  `gorm:"column:rule_id;primaryKey;autoIncrement"`
	MetricKey       string `gorm:"column:metric_key;type:varchar(64);not null;uniqueIndex:uniq_metric_threshold"`
	Threshold       int64  `gorm:"column:threshold;not null;uniqueIndex:uniq_metric_threshold"`
	RuleEnabled     bool   `gorm:"column:rule_enabled;not null"`
	ContentTemplate string `gorm:"column:content_template;type:text;not null"`
	CreatedAt       int64  `gorm:"column:created_at;not null"`
	UpdatedAt       int64  `gorm:"column:updated_at;not null"`
}

func (MilestoneRule) TableName() string { return "milestone_rules" }

type MilestoneEvent struct {
	EventID             int64  `gorm:"column:event_id;primaryKey"`
	ItemID              int64  `gorm:"column:item_id;not null;uniqueIndex:uniq_item_rule"`
	AuthorAgentID       int64  `gorm:"column:author_agent_id;not null"`
	RuleID              int64  `gorm:"column:rule_id;not null;uniqueIndex:uniq_item_rule"`
	MetricKey           string `gorm:"column:metric_key;type:varchar(64);not null"`
	Threshold           int64  `gorm:"column:threshold;not null"`
	CounterValue        int64  `gorm:"column:counter_value;not null"`
	NotificationContent string `gorm:"column:notification_content;type:text;not null"`
	NotificationStatus  int16  `gorm:"column:notification_status;type:smallint;not null;default:0"`
	QueuedAt            int64  `gorm:"column:queued_at;not null"`
	NotifiedAt          int64  `gorm:"column:notified_at;not null;default:0"`
	TriggeredAt         int64  `gorm:"column:triggered_at;not null"`
}

func (MilestoneEvent) TableName() string { return "milestone_events" }

type ItemContext struct {
	ItemID        int64  `gorm:"column:item_id"`
	AuthorAgentID int64  `gorm:"column:author_agent_id"`
	ItemSummary   string `gorm:"column:item_summary"`
}

func ListEnabledRulesByMetric(ctx context.Context, db *gorm.DB, metricKey string) ([]MilestoneRule, error) {
	var rules []MilestoneRule
	err := db.WithContext(ctx).
		Where("metric_key = ? AND rule_enabled = ?", metricKey, true).
		Order("threshold ASC, rule_id ASC").
		Find(&rules).Error
	return rules, err
}

func GetItemContext(ctx context.Context, db *gorm.DB, itemID int64) (*ItemContext, error) {
	var row ItemContext
	err := db.WithContext(ctx).
		Table("item_stats AS s").
		Select("s.item_id, s.author_agent_id, COALESCE(p.summary, '') AS item_summary").
		Joins("LEFT JOIN processed_items AS p ON p.item_id = s.item_id").
		Where("s.item_id = ?", itemID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertEventIfAbsent(ctx context.Context, db *gorm.DB, event *MilestoneEvent) (bool, error) {
	tx := db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "item_id"}, {Name: "rule_id"}},
			DoNothing: true,
		}).
		Create(event)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func ListPendingEvents(ctx context.Context, db *gorm.DB, limit int) ([]MilestoneEvent, error) {
	return ListPendingEventsAfter(ctx, db, 0, 0, limit)
}

func ListPendingEventsAfter(ctx context.Context, db *gorm.DB, afterQueuedAt, afterEventID int64, limit int) ([]MilestoneEvent, error) {
	var events []MilestoneEvent
	query := db.WithContext(ctx).
		Where("notification_status = ?", NotificationStatusPending).
		Order("queued_at ASC, event_id ASC")
	if afterQueuedAt > 0 || afterEventID > 0 {
		query = query.Where(
			"(queued_at > ?) OR (queued_at = ? AND event_id > ?)",
			afterQueuedAt,
			afterQueuedAt,
			afterEventID,
		)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&events).Error
	return events, err
}

func MarkEventsNotified(ctx context.Context, db *gorm.DB, eventIDs []int64, notifiedAt int64) error {
	if len(eventIDs) == 0 {
		return nil
	}
	return db.WithContext(ctx).
		Model(&MilestoneEvent{}).
		Where("event_id IN ?", eventIDs).
		Updates(map[string]interface{}{
			"notification_status": NotificationStatusNotified,
			"notified_at":         notifiedAt,
		}).Error
}
