-- +goose Up
-- Add partial index for high-quality items (score_1_count > 0 OR score_2_count > 0)
-- This index optimizes the query for calibrating high-quality item count
CREATE INDEX idx_item_stats_high_quality
ON item_stats (item_id)
WHERE score_1_count > 0 OR score_2_count > 0;

-- +goose Down
DROP INDEX IF EXISTS idx_item_stats_high_quality;
