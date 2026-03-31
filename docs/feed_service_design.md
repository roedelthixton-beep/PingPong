# Smart Feed Protocol and Deduplication Architecture Design

> Status: Active
> Last Updated: 2026-03-13

## 1. Architecture Background and Core Strategy

Traditional cursor-based pagination protocols using `update_time` are suitable for strict timeline replay systems (like chat history). However, this system relies on search engines and recommendation systems, with core requirements: "ensure content freshness, balance relevance, and filter duplicate content already seen by users."

To resolve the architectural conflict of "scoring systems cannot provide stable time cursors," this system abandons traditional cursor protocols and adopts a **"stateless client + strong server-side cache + global rolling deduplication"** design paradigm.

### Core Strategy

1. **Comprehensive Scoring Recall**: Search engine uses base relevance score × Gaussian time decay function, ensuring recall results match user interests while favoring recently published content.
2. **Global Rolling Bloom Filter**: Abandons per-user independent records, adopts daily/weekly rolling global bloom filters. Inactive users consume zero memory, naturally supporting "automatic expiration forgetting" of historical records.
3. **Read-Before-Cache**: After recalling large batches from the engine, immediately deduplicate, truncate and deliver clean data, store remaining in user-specific Redis List as pagination cache, greatly improving subsequent scroll-up loading performance.

## 2. Core Storage Layer Design (Redis)

### 2.1 Global Rolling Bloom Filter (Impression Deduplication)

Records all users' impression history within specific time periods.

- **Key Design**: `bf:global:{YYYYMMDD}` (daily rolling, e.g., `bf:global:20260306`)
- **Value Format**: `{agent_id}:{group_id}` (e.g., `u_10086:doc_9527`)
- **Lifecycle**: Keep last 7 days of keys, 8th day key automatically expires (implements "only deduplicate last 7 days seen" business logic)
- **Operations**: Relies on RedisBloom module's `BF.MADD` and `BF.MEXISTS`

### 2.2 User-Specific Pagination Cache (Delivery Buffer)

Stores user's current session "clean candidate set" after engine recall and deduplication.

- **Key Design**: `feed:cache:{agent_id}`
- **Data Structure**: Redis LIST
- **Value Format**: `group_id`
- **Lifecycle**: Short, e.g., 30 minutes expiration (forced re-recall after session expiration)

## 3. Client Interaction Protocol Design

Clients no longer maintain complex cursors or timestamps, only pass pull action and required quantity to server.

### 3.1 Request Structure

```json
{
  // Pull action
  // "refresh": pull-to-refresh, timed fetch, or cold start (requires latest data)
  // "load_more": scroll up for more (continue from current cache)
  "action": "refresh",

  // Expected items per request
  "limit": 20
}
```

### 3.2 Response Structure

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "id": "doc_9527",
        "title": "Latest High-Quality Content",
        "update_time": 1709658000
        // ... other business fields
      }
    ],
    // Indicates if server's delivery buffer has remaining data
    // Client can decide whether to show "no more" footer based on this field
    "has_more": true
  }
}
```

## 4. Core Interaction Flows

### Scenario 1: Pull-to-Refresh / Timed Fetch (action: "refresh")

This is the heaviest logic, responsible for fetching from search engine and cleaning.

1. **Clear Old Cache**: Server actively deletes user's current cache queue `DEL feed:cache:{agent_id}`
2. **Engine Recall**: Query search engine with time decay parameters, fetch Top N candidate `group_id` list (e.g., N=500)
3. **Batch Deduplication Check**:
   - Concatenate these 500 `group_id` into `{agent_id}:{group_id}` format
   - Use Redis Pipeline to concurrently execute `BF.MEXISTS` on last 7 days' 7 bloom filters
   - If any day's bloom filter returns `true`, remove that `group_id` from candidate set
4. **Truncate and Cache**:
   - Assume 300 "clean data" remain after deduplication
   - Truncate first `limit` items (e.g., 20) to prepare for client return
   - Write remaining 280 items via `RPUSH` to `feed:cache:{agent_id}`, set 30-minute expiration
5. **Record Impression**: Asynchronously write 20 items to be delivered via `BF.MADD` to today's global bloom filter `bf:global:{Today}`
6. **Assemble Return**: Extract `group_id` based on `limit`, fill content details, return to client, `has_more = true`

### Scenario 2: Scroll Up for More (action: "load_more")

This is the lightest logic, pure memory operation, extremely fast response.

1. **Direct Cache Read**: Server directly fetches next batch from Redis cache: `LPOP feed:cache:{agent_id} {limit}`
2. **Cache Empty Fallback**:
   - If `LPOP` returns 0 items (cache key expired or empty), current session cache exhausted, system automatically downgrades to `refresh`, silently executes full "Scenario 1" engine recall and filtering logic
   - If `LPOP` returns items > 0 but < `limit` (cache tail insufficient for one page), normally return popped data, `has_more = false`. Don't downgrade to avoid discarding already popped data
3. **Record Impression**: Asynchronously write `LPOP` popped data to today's global bloom filter `bf:global:{Today}`
4. **Assemble Return**: Fill content details and deliver

## 5. Edge Cases and Performance Optimization

- **False Positive Tolerance**: Bloom filters have extremely low probability (e.g., 1% or lower, depending on initialization parameters) of false positives. In recommendation feed scenarios, false positives only mean "small probability of missing one unread item," no substantial impact on overall user experience, reasonable engineering tradeoff.
- **Asynchronous Impression Write**: Writing records to bloom filter (`BF.MADD`) must be asynchronous (e.g., Go Goroutine or message queue), never block critical path of delivering data to client.
- **Client Timed Fetch Strategy**: For "fetch updates in background after some time" requirement, client only needs to silently initiate `action: "refresh"` request in background. If server returns non-empty `items` list, client can show red dot or "New" badge in UI to remind user; if empty, silently ignore.
