package milestone

import (
	"context"
	"sync"
	"time"

	milestonedal "pingpong_server/pkg/milestone/dal"

	"gorm.io/gorm"
)

const DefaultRuleCacheTTL = 60 * time.Second

type RuleCache struct {
	db  *gorm.DB
	ttl time.Duration
	now func() time.Time

	mu      sync.RWMutex
	entries map[string]cachedRules
}

type cachedRules struct {
	expiresAt time.Time
	rules     []milestonedal.MilestoneRule
}

func NewRuleCache(db *gorm.DB, ttl time.Duration) *RuleCache {
	if ttl <= 0 {
		ttl = DefaultRuleCacheTTL
	}
	return &RuleCache{
		db:      db,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cachedRules),
	}
}

func (c *RuleCache) GetEnabledRules(ctx context.Context, metricKey string) ([]milestonedal.MilestoneRule, error) {
	now := c.now()

	c.mu.RLock()
	entry, ok := c.entries[metricKey]
	c.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return cloneRules(entry.rules), nil
	}

	rules, err := milestonedal.ListEnabledRulesByMetric(ctx, c.db, metricKey)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[metricKey] = cachedRules{
		expiresAt: now.Add(c.ttl),
		rules:     cloneRules(rules),
	}
	c.mu.Unlock()

	return cloneRules(rules), nil
}

func (c *RuleCache) Invalidate(metricKey string) {
	if metricKey == "" {
		return
	}

	c.mu.Lock()
	delete(c.entries, metricKey)
	c.mu.Unlock()
}

func (c *RuleCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]cachedRules)
	c.mu.Unlock()
}

func cloneRules(rules []milestonedal.MilestoneRule) []milestonedal.MilestoneRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]milestonedal.MilestoneRule, len(rules))
	copy(out, rules)
	return out
}
