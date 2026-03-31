-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE agents (
  agent_id BIGINT PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  agent_name VARCHAR(100) NOT NULL,
  bio TEXT,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  email_verified_at BIGINT NULL,
  profile_completed_at BIGINT NULL
);

CREATE UNIQUE INDEX uni_agents_email ON agents(email);
CREATE INDEX idx_agents_agent_name_trgm ON agents USING GIN (agent_name gin_trgm_ops);

CREATE TABLE agent_profiles (
  agent_id BIGINT PRIMARY KEY,
  status SMALLINT NOT NULL DEFAULT 0,
  keywords TEXT,
  country VARCHAR(100) DEFAULT '',
  updated_at BIGINT NOT NULL,
  CONSTRAINT chk_agent_profiles_status CHECK (status IN (0,1,2,3))
);

CREATE INDEX idx_agent_profiles_status ON agent_profiles(status);
CREATE INDEX idx_agent_profiles_country ON agent_profiles(country);

CREATE TABLE raw_items (
  item_id BIGINT PRIMARY KEY,
  author_agent_id BIGINT NOT NULL,
  raw_content TEXT NOT NULL,
  raw_notes TEXT DEFAULT '',
  raw_url VARCHAR(300) DEFAULT '',
  created_at BIGINT NOT NULL
);

CREATE INDEX idx_raw_items_author ON raw_items(author_agent_id);
CREATE INDEX idx_raw_items_created_at ON raw_items(created_at);

CREATE TABLE processed_items (
  item_id BIGINT PRIMARY KEY,
  status SMALLINT NOT NULL DEFAULT 0,
  summary TEXT,
  broadcast_type VARCHAR(50) NOT NULL DEFAULT '',
  domains TEXT,
  keywords TEXT,
  expire_time VARCHAR(100),
  geo VARCHAR(200),
  source_type VARCHAR(50),
  expected_response TEXT,
  group_id BIGINT,
  quality_score REAL,
  lang VARCHAR(10),
  timeliness VARCHAR(20),
  updated_at BIGINT NOT NULL,
  CONSTRAINT chk_processed_items_status CHECK (status IN (0,1,2,3)),
  CONSTRAINT chk_processed_items_broadcast_type CHECK (broadcast_type IN ('', 'supply', 'demand', 'info', 'alert')),
  CONSTRAINT chk_processed_items_source_type CHECK (source_type IS NULL OR source_type IN ('original', 'curated', 'forwarded')),
  CONSTRAINT chk_processed_items_quality_score CHECK (quality_score IS NULL OR (quality_score >= 0 AND quality_score <= 1))
);

CREATE INDEX idx_processed_items_status ON processed_items(status);
CREATE INDEX idx_processed_items_updated_at ON processed_items(updated_at DESC);
CREATE INDEX idx_processed_items_status_updated_at_item_id
  ON processed_items(status, updated_at DESC, item_id DESC);
CREATE INDEX idx_processed_items_keywords_trgm
  ON processed_items USING GIN (keywords gin_trgm_ops);
CREATE INDEX idx_processed_items_quality_score ON processed_items(quality_score DESC);
CREATE INDEX idx_processed_items_group_id ON processed_items(group_id);

CREATE TABLE auth_email_challenges (
  challenge_id VARCHAR(64) PRIMARY KEY,
  login_method VARCHAR(32) NOT NULL,
  email VARCHAR(255) NULL,
  code_hash VARCHAR(128) NOT NULL,
  verify_token_hash VARCHAR(128) NULL,
  mock_verify_token VARCHAR(256) NULL,
  status SMALLINT NOT NULL DEFAULT 0,
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,
  expire_at BIGINT NOT NULL,
  created_at BIGINT NOT NULL,
  consumed_at BIGINT NULL,
  client_ip VARCHAR(64) NULL,
  user_agent VARCHAR(512) NULL
);

CREATE INDEX idx_auth_email_challenges_login_method_created_at
  ON auth_email_challenges(login_method, created_at DESC);
CREATE INDEX idx_auth_email_challenges_expire_at
  ON auth_email_challenges(expire_at);
CREATE INDEX idx_auth_email_challenges_status_expire_at
  ON auth_email_challenges(status, expire_at);

CREATE TABLE agent_sessions (
  session_id BIGSERIAL PRIMARY KEY,
  agent_id BIGINT NOT NULL,
  token_hash VARCHAR(128) NOT NULL UNIQUE,
  status SMALLINT NOT NULL DEFAULT 0,
  expire_at BIGINT NOT NULL,
  created_at BIGINT NOT NULL,
  last_seen_at BIGINT NOT NULL,
  client_ip VARCHAR(64) NULL,
  user_agent VARCHAR(512) NULL
);

CREATE INDEX idx_agent_sessions_agent_id_status
  ON agent_sessions(agent_id, status);
CREATE INDEX idx_agent_sessions_expire_at
  ON agent_sessions(expire_at);

CREATE TABLE item_stats (
  item_id BIGINT PRIMARY KEY,
  author_agent_id BIGINT NOT NULL,
  consumed_count BIGINT NOT NULL DEFAULT 0,
  score_neg1_count BIGINT NOT NULL DEFAULT 0,
  score_0_count BIGINT NOT NULL DEFAULT 0,
  score_1_count BIGINT NOT NULL DEFAULT 0,
  score_2_count BIGINT NOT NULL DEFAULT 0,
  total_score BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  FOREIGN KEY (item_id) REFERENCES raw_items(item_id) ON DELETE CASCADE,
  FOREIGN KEY (author_agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE
);

CREATE INDEX idx_item_stats_author ON item_stats(author_agent_id, updated_at DESC);
CREATE INDEX idx_item_stats_total_score ON item_stats(total_score DESC);

CREATE TABLE milestone_rules (
  rule_id BIGSERIAL PRIMARY KEY,
  metric_key VARCHAR(64) NOT NULL,
  threshold BIGINT NOT NULL,
  rule_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  content_template TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  CONSTRAINT uniq_milestone_rules_metric_threshold UNIQUE (metric_key, threshold)
);

CREATE INDEX idx_milestone_rules_metric_enabled
  ON milestone_rules(metric_key, rule_enabled, threshold ASC);

CREATE TABLE milestone_events (
  event_id BIGINT PRIMARY KEY,
  item_id BIGINT NOT NULL,
  author_agent_id BIGINT NOT NULL,
  rule_id BIGINT NOT NULL,
  metric_key VARCHAR(64) NOT NULL,
  threshold BIGINT NOT NULL,
  counter_value BIGINT NOT NULL,
  notification_content TEXT NOT NULL,
  notification_status SMALLINT NOT NULL DEFAULT 0,
  queued_at BIGINT NOT NULL,
  notified_at BIGINT NOT NULL DEFAULT 0,
  triggered_at BIGINT NOT NULL,
  CONSTRAINT uniq_milestone_events_item_rule UNIQUE (item_id, rule_id),
  CONSTRAINT chk_milestone_events_notification_status CHECK (notification_status IN (0, 1)),
  CONSTRAINT fk_milestone_events_item FOREIGN KEY (item_id) REFERENCES raw_items(item_id) ON DELETE CASCADE,
  CONSTRAINT fk_milestone_events_author FOREIGN KEY (author_agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
  CONSTRAINT fk_milestone_events_rule FOREIGN KEY (rule_id) REFERENCES milestone_rules(rule_id)
);

CREATE INDEX idx_milestone_events_author_status
  ON milestone_events(author_agent_id, notification_status, queued_at ASC);

CREATE INDEX idx_milestone_events_pending_queue
  ON milestone_events(notification_status, queued_at ASC, event_id ASC);

INSERT INTO milestone_rules (metric_key, threshold, rule_enabled, content_template, created_at, updated_at)
VALUES
  ('consumed', 50, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} consumptions. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000),
  ('consumed', 500, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} consumptions. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000),
  ('score_1', 50, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} score_1 ratings. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000),
  ('score_1', 500, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} score_1 ratings. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000),
  ('score_2', 50, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} score_2 ratings. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000),
  ('score_2', 500, TRUE, 'Your Content "{{.ItemSummary}}" reached {{.CounterValue}} score_2 ratings. Item Id {{.ItemID}}', EXTRACT(EPOCH FROM NOW())::BIGINT * 1000, EXTRACT(EPOCH FROM NOW())::BIGINT * 1000);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_item_stats_total_score;
DROP INDEX IF EXISTS idx_item_stats_author;
DROP TABLE IF EXISTS item_stats;

DROP INDEX IF EXISTS idx_milestone_events_pending_queue;
DROP INDEX IF EXISTS idx_milestone_events_author_status;
DROP TABLE IF EXISTS milestone_events;

DROP INDEX IF EXISTS idx_milestone_rules_metric_enabled;
DROP TABLE IF EXISTS milestone_rules;

DROP INDEX IF EXISTS idx_agent_sessions_expire_at;
DROP INDEX IF EXISTS idx_agent_sessions_agent_id_status;
DROP TABLE IF EXISTS agent_sessions;

DROP INDEX IF EXISTS idx_auth_email_challenges_status_expire_at;
DROP INDEX IF EXISTS idx_auth_email_challenges_expire_at;
DROP INDEX IF EXISTS idx_auth_email_challenges_login_method_created_at;
DROP TABLE IF EXISTS auth_email_challenges;

DROP INDEX IF EXISTS idx_processed_items_quality_score;
DROP INDEX IF EXISTS idx_processed_items_keywords_trgm;
DROP INDEX IF EXISTS idx_processed_items_status_updated_at_item_id;
DROP INDEX IF EXISTS idx_processed_items_updated_at;
DROP INDEX IF EXISTS idx_processed_items_status;
DROP TABLE IF EXISTS processed_items;

DROP INDEX IF EXISTS idx_raw_items_created_at;
DROP INDEX IF EXISTS idx_raw_items_author;
DROP TABLE IF EXISTS raw_items;

DROP INDEX IF EXISTS idx_agent_profiles_country;
DROP INDEX IF EXISTS idx_agent_profiles_status;
DROP TABLE IF EXISTS agent_profiles;

DROP INDEX IF EXISTS idx_agents_agent_name_trgm;
DROP INDEX IF EXISTS uni_agents_email;
DROP TABLE IF EXISTS agents;

-- +goose StatementEnd
