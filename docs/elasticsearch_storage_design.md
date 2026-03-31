# Elasticsearch Storage Design and Scaling Operations Strategy

> Status: Active
> Last Updated: 2026-03-13

## 1. Overview

This document describes the Elasticsearch storage architecture design, ILM (Index Lifecycle Management) lifecycle management strategy, and scaling operations plan for the pingpong_server project.

### 1.1 Design Objectives

- **Automated Lifecycle Management**: Hot data → Warm data → Cold data three-stage automatic transition
- **Storage Cost Optimization**: Reduce storage and memory costs through force_merge, replica reduction, and read-only archiving
- **Query Performance Guarantee**: Hot data high priority, fast response; cold data low priority, resource saving
- **Business Code Transparency**: Implement Rollover through Index Alias, business code doesn't need to be aware of underlying index changes
- **Vector Search Support**: `dense_vector` field dimensions automatically inferred from `EMBEDDING_DIMENSIONS` or `EMBEDDING_MODEL`, supports semantic search based on cosine similarity
- **Flexible Configuration**: Dynamically configure shard and replica counts via environment variables, adapting to different deployment environments

### 1.2 Technology Stack

- Elasticsearch 8.11.0
- ILM (Index Lifecycle Management)
- Composable Index Templates
- Index Aliases with Rollover
- Dense Vector Search (kNN)

---

## 2. Code Review Results

### 2.1 Compilation Correctness ✅

**Checked Items**:
- Package import completeness
- Type matching
- Function signature consistency
- Constant reference correctness

**Conclusion**: All code compiles successfully, no syntax errors.

**Key Checkpoints**:
- `IndexMapping` in `pkg/es/ilm.go` correctly references from `mapping.go`
- `es.ReadIndexPattern` and `es.IndexName` constant references correct in `rpc/sort/dal/*.go`
- ES official Go client API calls comply with v8 specifications

### 2.2 Code Logic Reasonableness ✅

#### 2.2.1 ILM Policy Design (ilm.go)

**Advantages**:
1. **Idempotency Guarantee**: `upsertILMPolicy` and `upsertIndexTemplate` use PUT operations, repeated execution won't error
2. **Safe Bootstrap**: `bootstrapIfNeeded` doesn't force delete when old index exists, avoiding data loss
3. **Clear Error Handling**: Each step has explicit error return and logging
4. **Phased Initialization**: Policy → Template → Index, reasonable sequence
5. **Environment Variable Configuration**: Dynamically configure via `ES_SHARDS` and `ES_REPLICAS` environment variables, adapting to different deployment environments
6. **Enhanced Idempotency**: Bootstrap checks if initial index `items-000001` already exists, avoiding duplicate creation

**Implemented Optimizations**:
1. ✅ **Dynamic Replica Configuration**: Configure via `ES_REPLICAS` environment variable, default 0 (single node), multi-node environments can set to 1
2. ✅ **Bootstrap Idempotency Enhancement**: Added `items-000001` existence check, avoiding duplicate creation in extreme cases
3. ✅ **Warm Phase Optimization**: `force_merge` + `replicas=0`, reducing segment count and replica memory usage

#### 2.2.2 Read-Write Separation (es_query.go, es_similarity.go, es.go)

**Advantages**:
1. **Write Transparency**: All write operations (IndexItem, BulkIndexItems) use `es.IndexName` (alias), automatically route to new index after Rollover
2. **Read All**: All read operations (SearchItems, SearchSimilarItems) use `es.ReadIndexPattern` (`items-*`), query across all backing indices
3. **Delete Compatibility**: `DeleteItem` uses `delete_by_query` API, supports multi-index scenarios

**Key Fix**:
- Original `DeleteItem` used single document DELETE API, would error when alias points to multiple indices after Rollover
- Fixed to `delete_by_query`, precisely matches `id` field through `term` query

#### 2.2.3 Vector Search (es_similarity.go)

**Advantages**:
1. **Cross-Index Vector Search**: Uses `ReadIndexPattern` ensuring similarity deduplication can search full historical data
2. **Cosine Similarity**: `dense_vector` field configured with `cosine` similarity, suitable for semantic vectors
3. **Dimension Consistency**: Index dimensions must match current embedding model output dimensions; switching models requires index rebuild or migration

**Notes**:
- Vector indices resident in memory, Warm/Cold phase indices still occupy memory
- ES 8.x doesn't support dynamically disabling field indexing, can only reduce memory usage through `replicas=0` + `force_merge`

### 2.3 ES Schema Design Reasonableness ✅

#### 2.3.1 Field Mapping (mapping.go)

```go
"properties": {
    "id":               {"type": "keyword"},           // Exact match
    "raw_content":      {"type": "text"},              // Full-text search
    "summary":          {"type": "text"},
    "keywords":         {"type": "text", "analyzer": "keyword_analyzer"},  // Comma-separated keywords
    "domains":          {"type": "text", "analyzer": "keyword_analyzer"},
    "embedding":        {"type": "dense_vector", "dims": EMBEDDING_DIMENSIONS, "similarity": "cosine"},
    "created_at":       {"type": "date"},
    "updated_at":       {"type": "date"},
    ...
}
```

**Design Assessment**:
1. **`id` field**: `keyword` type, supports exact match and `term` query, correct
2. **`keywords` and `domains`**: Use `keyword_analyzer` (lowercase + keyword tokenizer), suitable for comma-separated tag matching
3. **`embedding` field**: `dense_vector` with dimensions determined by current embedding configuration, `cosine` similarity, meets semantic search needs
4. **Time fields**: `date` type, supports range queries and sorting

**Potential Optimization**:
- `keywords` and `domains` could be changed to `keyword` array type, avoiding comma-separated string parsing overhead
  - Current design: `"AI,Machine Learning,NLP"` → requires application layer `strings.Split`
  - Optimization: `["AI", "Machine Learning", "NLP"]` → ES native array support
  - Impact: Requires modifying DAL layer and database schema

#### 2.3.2 Index Settings (ilm.go)

```go
"settings": {
    "number_of_shards":   getEnvInt("ES_SHARDS", 1),
    "number_of_replicas": getEnvInt("ES_REPLICAS", 0),
    "refresh_interval":   "30s",
    ...
}
```

**Design Assessment**:
1. **`number_of_shards`**:
   - Default: 1 (suitable for single node or small data < 50GB/index)
   - Scalability: Each backing index independent after Rollover, can increase concurrency by adding nodes
   - Configuration: Adjust via `ES_SHARDS` environment variable (production recommended `shards = node count`)

2. **`number_of_replicas`**:
   - Default: 0 (suitable for single node deployment or test environment)
   - Risk: No replicas, node failure will lose data
   - Configuration: Adjust via `ES_REPLICAS` environment variable (production recommended set to 1, requires at least 2 nodes)

3. **`refresh_interval: 30s`**:
   - Advantage: Reduces write pressure, improves throughput
   - Disadvantage: Up to 30 seconds after write before searchable
   - Use case: Non-real-time query scenarios (like feed streams)
   - Recommendation: Change to `1s` (ES default) for high real-time requirements

---

## 3. ILM Lifecycle Strategy

### 3.1 Three-Phase Design

| Phase | Time Range | Rollover Condition | Main Operations | Resource Priority |
|-------|------------|-------------------|-----------------|-------------------|
| **Hot** | 0-7 days | `max_age: 7d` OR `max_size: 20gb` | Accept writes, real-time queries, vector search | 100 (highest) |
| **Warm** | 7-90 days | - | `force_merge` (merge segments), `replicas=0`, `readonly` | 50 (medium) |
| **Cold** | 90+ days | - | `replicas=0`, `readonly`, low priority | 0 (lowest) |

### 3.2 Hot Phase (0-7 days)

**Objective**: High-performance writes and queries

**Configuration**:
```json
{
  "actions": {
    "rollover": {
      "max_age": "7d",
      "max_size": "20gb"
    },
    "set_priority": {"priority": 100}
  }
}
```

**Behavior**:
- Write alias `items` points to current Hot index (`is_write_index: true`)
- When Rollover condition met, automatically creates new index (e.g., `items-000002`), alias switches to new index
- Old index enters Warm phase

**Rollover Trigger Conditions**:
- Time: 7 days after index creation
- Size: Index size reaches 20GB
- **Either condition triggers**

### 3.3 Warm Phase (7-90 days)

**Objective**: Reduce storage and memory costs, maintain queryability

**Configuration**:
```json
{
  "min_age": "7d",
  "actions": {
    "forcemerge": {"max_num_segments": 1},
    "allocate": {"number_of_replicas": 0},
    "readonly": {},
    "set_priority": {"priority": 50}
  }
}
```

**Behavior**:
1. **Force Merge**: Merge all segments into 1, reducing file count and memory usage
2. **Replica Reduction**: `replicas=0`, freeing replica storage and memory
3. **Read-only Protection**: Prevent accidental writes
4. **Priority Reduction**: Prioritize Hot index during resource competition

**Notes**:
- Force Merge is CPU-intensive, recommended during off-peak hours
- Vector indices still occupy memory (ES 8.x can't dynamically disable field indexing)

### 3.4 Cold Phase (90+ days)

**Objective**: Minimize resource usage, archive historical data

**Configuration**:
```json
{
  "min_age": "90d",
  "actions": {
    "allocate": {"number_of_replicas": 0},
    "readonly": {},
    "set_priority": {"priority": 0}
  }
}
```

**Behavior**:
- Read-only, no replicas, lowest priority
- Query performance lower, but still queryable via `items-*` pattern

**Optional Extensions**:
- **Searchable Snapshot**: Snapshot index to object storage (S3/GCS), further reducing local storage costs
- **Requires configuring Snapshot Repository** (not implemented in current code)

---

## 4. Index Naming and Alias Architecture

### 4.1 Naming Convention

```
Write alias:  items              → Points to current Hot index (is_write_index: true)
Read pattern: items-*            → Matches all backing indices
Actual index: items-000001       → Initial index
             items-000002       → 2nd index after Rollover
             items-000003       → 3rd index after Rollover
             ...
```

### 4.2 Read-Write Separation

| Operation Type | Use Index/Alias | Description |
|----------------|-----------------|-------------|
| **Write** | `items` | Alias automatically routes to current Hot index |
| **Read** | `items-*` | Query across all backing indices |
| **Delete** | `items-*` | Use `delete_by_query` for cross-index deletion |

### 4.3 Rollover Flow

```
Initial state:
  items (alias) → items-000001 (is_write_index: true)

After Rollover triggered:
  items (alias) → items-000002 (is_write_index: true)
                  items-000001 (is_write_index: false, enters Warm phase)

When querying:
  items-* matches items-000001 and items-000002, returns merged results
```

---

## 5. Environment Variable Configuration

### 5.1 Configuration Items

| Environment Variable | Default | Description | Use Case |
|---------------------|---------|-------------|----------|
| `ES_SHARDS` | 1 | Primary shards per index | Single node: 1; Multi-node: node count |
| `ES_REPLICAS` | 0 | Replicas per index | Single node: 0; Multi-node: 1 |

### 5.2 Configuration Examples

**Single Node Deployment (Dev/Test Environment)**:
```bash
ES_SHARDS=1
ES_REPLICAS=0
```

**Multi-Node Deployment (Production, 3 nodes)**:
```bash
ES_SHARDS=3
ES_REPLICAS=1
```

**Notes**:
- Configuration changes require service restart, new config only applies to newly created indices
- Existing indices won't automatically update config, need manual adjustment or wait for Rollover

---

## 6. Scaling and Operations Strategy

### 6.1 Horizontal Scaling

#### 6.1.1 Single Node → Multi-Node

**Steps**:
1. Set environment variables:
   ```bash
   export ES_REPLICAS=1
   export ES_SHARDS=3  # Assuming 3 nodes
   ```
2. Add ES nodes (at least 2 nodes)
3. Restart service, new indices automatically apply new config
4. Manually update old indices replica count:
   ```bash
   curl -X PUT "localhost:9200/items-*/_settings" -H 'Content-Type: application/json' -d'
   {
     "index": {
       "number_of_replicas": 1
     }
   }'
   ```

#### 6.1.2 Increase Shard Count

**Scenario**: Single index data exceeds 50GB, query performance degrades

**Solution**:
1. Set environment variable:
   ```bash
   export ES_SHARDS=3
   ```
2. Restart service, new Rollover indices automatically apply new config
3. Old indices can't modify shard count (ES limitation), can only migrate via Reindex

**Recommendations**:
- Initial shard count = node count (e.g., 3 nodes → 3 shards)
- Single shard size controlled at 20-50GB

### 6.2 Storage Scaling

#### 6.2.1 Disk Space Insufficient

**Temporary Solution**:
1. Manually delete Cold phase indices:
   ```bash
   curl -X DELETE "localhost:9200/items-000001"
   ```
2. Adjust Rollover conditions (e.g., `max_size: 10gb`)

**Long-term Solution**:
1. Configure Snapshot Repository, enable Searchable Snapshot
2. Cold phase indices automatically snapshot to object storage

#### 6.2.2 High Memory Usage

**Cause**: Vector indices resident in memory

**Optimization Solutions**:
1. Warm phase `force_merge` + `replicas=0`, reduce segment count and replica memory
2. Reduce Hot index count (shorten Rollover cycle)
3. Upgrade to larger memory nodes

### 6.3 Performance Optimization

#### 6.3.1 Write Performance

**Current Configuration**:
- `refresh_interval: 30s`: Reduce refresh frequency, improve throughput
- `number_of_replicas: 0` (default): No replica writes, reduce network overhead

**Further Optimization**:
- Bulk writes: Use `BulkIndexItems` (already implemented)
- Add `index.translog.durability: async` (risk: node crash may lose data)

#### 6.3.2 Query Performance

**Current Configuration**:
- Hot index `priority: 100`, prioritize resource allocation
- Warm index `force_merge`, reduce segment count

**Further Optimization**:
- Use `_routing` parameter, route same `author_id` documents to same shard
- Enable query cache: `index.queries.cache.enabled: true`

#### 6.3.3 Vector Search Performance

**Current Configuration**:
- Dynamic dimension `dense_vector`, `cosine` similarity
- kNN query across all `items-*` indices

**Optimization Solutions**:
- Limit vector search range: Only query Hot + Warm indices (e.g., `items-00000[2-9]*`)
- Use `num_candidates` parameter to control candidate count (default 10000)

---

## 7. Monitoring and Alerting

### 7.1 Key Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `indices.count` | Index count | > 100 (check Rollover frequency) |
| `indices.store.size` | Total storage size | > 80% disk capacity |
| `jvm.mem.heap_used_percent` | JVM heap memory usage | > 85% |
| `indices.search.query_time_in_millis` | Query latency | > 1000ms (P99) |
| `indices.indexing.index_time_in_millis` | Write latency | > 500ms (P99) |

### 7.2 ILM Status Check

```bash
# View ILM policy
curl "localhost:9200/_ilm/policy/items-policy?pretty"

# View index ILM status
curl "localhost:9200/items-*/_ilm/explain?pretty"

# View alias bindings
curl "localhost:9200/_cat/aliases/items?v"

# View all backing indices
curl "localhost:9200/_cat/indices/items-*?v&s=index"
```

### 7.3 Common Issue Troubleshooting

#### 7.3.1 Rollover Not Triggered

**Check**:
```bash
curl "localhost:9200/items-*/_ilm/explain?pretty" | grep -A 5 "phase"
```

**Possible Causes**:
- ILM policy not bound to index
- Rollover conditions not met (time < 7d and size < 20GB)
- ILM service not started

**Solution**:
```bash
# Manually trigger Rollover
curl -X POST "localhost:9200/items/_rollover?pretty"
```

#### 7.3.2 Query Error "alias has more than one index"

**Cause**: Using single document API (GET/DELETE) on alias, but alias points to multiple indices

**Solution**:
- GET → Change to Search API
- DELETE → Change to `delete_by_query` API (already fixed)

#### 7.3.3 Incomplete Vector Search Results

**Cause**: Only queried `items` alias (current Hot index), didn't query historical indices

**Solution**:
- Ensure using `items-*` pattern query (already fixed)

---

## 8. Migration Guide

### 8.1 Migrate from Old Static Index

**Scenario**: Existing `items` index (not alias), need to migrate to ILM management

**Steps**:
1. **Backup data** (optional):
   ```bash
   curl -X PUT "localhost:9200/_snapshot/my_backup/snapshot_1?wait_for_completion=true"
   ```

2. **Delete old index**:
   ```bash
   curl -X DELETE "localhost:9200/items"
   ```

3. **Restart service**:
   ```bash
   ./scripts/local/start_local.sh
   ```
   - Service automatically executes `SetupILM` on startup
   - Creates `items-000001` and binds `items` alias

4. **Verify**:
   ```bash
   curl "localhost:9200/_cat/aliases/items?v"
   curl "localhost:9200/_cat/indices/items-*?v"
   ```

### 8.2 Data Reindex (Optional)

**Scenario**: Need to migrate old data to new ILM-managed index

**Steps**:
```bash
curl -X POST "localhost:9200/_reindex?pretty" -H 'Content-Type: application/json' -d'
{
  "source": {
    "index": "items_backup"
  },
  "dest": {
    "index": "items"
  }
}
'
```

---

## 9. Summary

### 9.1 Design Advantages

1. **Automated Lifecycle Management**: No manual intervention, indices automatically Rollover and downgrade
2. **Cost Optimization**: Warm/Cold phases reduce storage and memory costs through force_merge, replica reduction
3. **Business Transparency**: Read-write separation through aliases, business code doesn't need modification
4. **Strong Scalability**: Supports horizontal scaling (add nodes) and vertical scaling (add shards)
5. **Vector Search Support**: Dynamic dimension dense_vector, supports semantic similarity search
6. **Flexible Configuration**: Dynamic configuration via environment variables, adapts to different deployment environments

### 9.2 Implemented Optimizations

1. ✅ **Dynamic Replica Configuration**: Configure via `ES_REPLICAS` environment variable, default 0 (single node), multi-node environments can set to 1
2. ✅ **Dynamic Shard Configuration**: Configure via `ES_SHARDS` environment variable, default 1
3. ✅ **Bootstrap Idempotency Enhancement**: Added `items-000001` existence check, avoiding duplicate creation in extreme cases
4. ✅ **Warm Phase Optimization**: `force_merge` + `replicas=0`, reducing segment count and replica memory usage

### 9.3 Notes

1. **Configuration Effective Timing**: Environment variable changes require service restart, new config only applies to newly created indices
2. **Single Node Limitation**: `replicas=0` no data redundancy, production recommended at least 2 nodes + `replicas=1`
3. **Vector Memory Usage**: Warm/Cold phase vector indices still occupy memory, need to monitor JVM heap usage
4. **Refresh Delay**: `refresh_interval: 30s` causes up to 30 seconds visibility after write, high real-time scenarios need adjustment
5. **Rollover Frequency**: Current 7 days or 20GB, need to adjust based on actual data volume

### 9.4 Future Optimization Directions

1. **Searchable Snapshot**: Configure Snapshot Repository, Cold phase indices snapshot to object storage
2. **Keyword Field Optimization**: Change `keywords` and `domains` to `keyword` array type
3. **Query Cache**: Enable `index.queries.cache.enabled` to improve repeated query performance
4. **Monitoring and Alerting**: Integrate Prometheus + Grafana, monitor ILM status and performance metrics

---

## 10. References

- [Elasticsearch ILM Official Documentation](https://www.elastic.co/guide/en/elasticsearch/reference/8.11/index-lifecycle-management.html)
- [Composable Index Templates](https://www.elastic.co/guide/en/elasticsearch/reference/8.11/index-templates.html)
- [Dense Vector Search](https://www.elastic.co/guide/en/elasticsearch/reference/8.11/dense-vector.html)
- [Index Aliases](https://www.elastic.co/guide/en/elasticsearch/reference/8.11/aliases.html)

---

**Document Version**: v1.0  
**Last Updated**: 2026-03-13  
**Maintainer**: pingpong_server Development Team
