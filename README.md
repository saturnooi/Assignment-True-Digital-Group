# Recommendation Service

A production-ready backend recommendation service built with Go, PostgreSQL, and Redis.

---

## 14.1 Setup Instructions

### Prerequisites

| Dependency     | Version                          |
| -------------- | -------------------------------- |
| Go             | 1.25+                            |
| Docker         | 24+                              |
| Docker Compose | v2+                              |
| k6             | latest (for performance testing) |


> No local PostgreSQL or Redis installation is required — all services run inside Docker.

### Step-by-Step Installation

**1. Clone the repository**

```bash
git clone <repository-url>
cd assignment
```

**2. Start all services with one command**

```bash
docker-compose up --build
```

Or using the Makefile shortcut:

```bash
make up
```

This will automatically:
1. Start PostgreSQL 15 and wait until healthy
2. Run database migrations (`migrate` container)
3. Seed initial data (`seed` container — 2,000 users, 5,000 content, 20,000 watch history records)
4. Start Redis 7
5. Start the API server

**3. Verify the service is running**

```bash
curl http://localhost:8080/users/1/recommendations?limit=10
```

### Commands Reference

| Action               | Makefile       | Docker Compose                  |
| -------------------- | -------------- | ------------------------------- |
| Start all services   | `make up`      | `docker-compose up --build`     |
| Stop all services    | `make down`    | `docker-compose down`           |
| Full reset (volumes) | `make reset`   | `docker-compose down -v`        |
| Run migrations only  | `make migrate` | `docker-compose up migrate`     |
| Run seed only        | `make seed`    | `docker-compose up seed`        |
| Start database only  | `make db`      | `docker-compose up -d postgres` |
| Start Redis only     | `make redis`   | `docker-compose up -d redis`    |
| Stream API logs      | `make logs`    | `docker-compose logs -f app`    |

### API Endpoints

```
GET  /users/{user_id}/recommendations?limit=10
POST /users/{user_id}/watch-history            Body: {"content_id": 123}
GET  /recommendations/batch?page=1&limit=20
```

---

## 14.2 Architecture Overview

### High-Level System Design

```
┌─────────────────────────────────────────┐
│             Handler Layer               │
│    (HTTP, Validation, Serialization)    │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│             Service Layer               │
│    (Business Logic, Orchestration)      │
└──────┬───────────────────────┬──────────┘
       │                       │
       ▼                       ▼
┌──────────────┐      ┌─────────────────┐
│  Repository  │      │  Model Client   │
│ (Data Access)│      │   (Scoring)     │
└──────┬───────┘      └─────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│       Database + Cache Layer            │
│        (PostgreSQL + Redis)             │
└─────────────────────────────────────────┘
```

### Explanation of Each Architectural Layer

**Handler Layer** — `internal/adapter/handler/`, `internal/adapter/http/`

Receives HTTP requests, validates input parameters (`user_id`, `limit`, `page`), calls the appropriate usecase method, and serializes the JSON response. Contains no business logic. A global error handler maps application errors to HTTP status codes.

**Service Layer** — `internal/usecase/`

Orchestrates the recommendation workflow: cache lookup, database fetching, genre preference computation, model invocation, result sorting, and cache store. Two usecases:
- `UserUsecase` — single-user recommendations with a 500 ms request timeout; `RecordWatchHistory` records new watch events and invalidates the corresponding cache keys
- `RecommendationUsecase` — batch recommendations via a bounded worker pool with a 30 s overall timeout

**Repository Layer** — `internal/adapter/repository/`

Abstracts all PostgreSQL queries. Implements the `port.UserRepository` interface. Queries include paginated user fetching, watch history with genre JOIN, unwatched content exclusion, and bulk history loading to avoid N+1 queries.

**Model Client** — `internal/adapter/model/`

Implements `port.ModelClient`. Applies a heuristic scoring function over candidate content using popularity, genre preference, recency, and controlled randomness. Simulates 30–50 ms latency and a 1.5% random failure rate to reflect real ML integration behavior.

**Cache Layer** — `internal/adapter/cache/`

Redis-backed `redisCache` struct that implements the `port.Cache` interface. Injected directly into `UserUsecase` via the constructor — no context-passing or middleware required. Provides `Get`, `Set`, `Del`, and `DelPattern` operations. `DelPattern` uses Redis `SCAN` to atomically delete all cache keys matching a glob pattern (e.g. `rec:user:{id}:limit:*`), enabling active invalidation when a user's watch history is updated.

**Domain Layer** — `internal/domain/`

Pure Go structs with no infrastructure dependencies: `User`, `Content`, `WatchRecord`, `ScoredContent`. These are the only types passed between layers.

**Port Layer** — `internal/port/`

Go interfaces that decouple business logic from infrastructure implementations:
- `UserRepository` — all database access methods including `GetUsersByIDs` for bulk fetching and `RecordWatch` for inserting watch events
- `ModelClient` — recommendation scoring contract
- `Cache` — cache read/write/delete/pattern-delete contract (`Get`, `Set`, `Del`, `DelPattern`)

All three interfaces enable independent testing and future substitution without touching business logic.

### Data Flow Through the System

**Single User Recommendation Flow**

```
1. Client → GET /users/{id}/recommendations?limit=10
2. Handler validates user_id (must be > 0) and limit (default 10, max 50)
3. UserUsecase sets 500 ms deadline, checks Redis: key = rec:user:{id}:limit:{limit}
   ├── Cache HIT  → deserialize, set cache_hit=true, return immediately
   └── Cache MISS → continue below
4. Repository queries PostgreSQL:
   a. SELECT user from users WHERE id=$1
   b. SELECT c.id, c.genre, uwh.watched_at FROM user_watch_history JOIN content ... LIMIT 50
   c. SELECT id, title, genre, popularity_score, created_at FROM content
      WHERE NOT EXISTS (watched) ORDER BY popularity_score DESC LIMIT 100
5. Usecase builds genre preference map (normalized frequency)
6. ModelClient.ScoreCandidates() applies scoring formula, sleeps 30–50 ms
7. Results sorted descending by score, truncated to limit
8. Response serialized and stored in Redis (TTL: 10 min)
9. JSON response returned to client

Watch History Recording Flow

1. Client → POST /users/{id}/watch-history  Body: {"content_id": 123}
2. Handler validates user_id and content_id
3. UserUsecase.RecordWatchHistory inserts row into user_watch_history
4. Cache invalidation: DelPattern("rec:user:{id}:limit:*") removes all cached
   recommendation sets for this user via Redis SCAN + DEL
5. Returns {"status": "ok"}
```

**Batch Recommendation Flow**

```
1. Client → GET /recommendations/batch?page=1&limit=20
2. Handler validates page (min 1) and limit (default 20, max 100)
3. Repository fetches paginated user IDs + total count (two queries)
4. Repository bulk-fetches full user profiles via GetUsersByIDs (WHERE id = ANY($1))
5. Repository bulk-loads watch histories for all user IDs via ANY($1) — avoids N+1
6. Repository loads top 100 content once (shared across all workers)
7. Bounded worker pool: workers = min(limit, NumCPU × 2)
8. Workers process users concurrently via job channel:
   - Look up real domain.User from usersMap
   - Build genre preference map per user
   - Build per-user watched-ID set from historyMap; filter candidates in-memory
   - Call ModelClient.ScoreCandidates(user, filteredCandidates, pref)
   - Sort, take top 10
   - Send BatchResult to results channel
9. Aggregate: count success/failure, measure processing_time_ms
10. Return BatchResponse with pagination info, results, summary, BatchMetadata
```

### How the Recommendation Model Integrates with Database Queries

The Repository layer fetches structured data that the Model Client consumes directly:

```go
// Repository provides (single-user):
user         domain.User        // age, country, subscription from users table
history      []domain.WatchRecord  // c.id, c.genre, uwh.watched_at — last 50 records
candidates   []domain.Content      // unwatched, top 100 by popularity_score DESC

// Repository provides (batch — all fetched in bulk to avoid N+1):
usersMap     map[int64]domain.User          // GetUsersByIDs: WHERE id = ANY($1)
historyMap   map[int64][]domain.WatchRecord // GetWatchHistoryByUserIDs: WHERE user_id = ANY($1)
candidates   []domain.Content               // GetTopContent: shared across all workers

// Per-user in worker (no extra DB query):
watchedIDs   map[int64]struct{}             // built from historyMap[userID] entries
filtered     []domain.Content               // candidates with watched items removed

// Usecase derives per-user:
genrePreferences map[string]float64  // normalized genre frequency from watch history

// Model Client contract:
ScoreCandidates(user domain.User, candidates []domain.Content, genrePreferences map[string]float64) ([]ScoredContent, error)
```

---

## 14.3 Design Decisions

### Caching Strategy and TTL Rationale

**Cache key design:**

```
rec:user:{user_id}:limit:{limit}
```

Both `user_id` and `limit` are included in the key because recommendations differ per user and per requested count. Storing the full serialized JSON response avoids recomputation entirely on cache hit.

**TTL: 10 minutes**

| TTL Strategy            | Trade-off                                                     |
|-------------------------|--------------------------------------------------------------|
| Too short (< 1 min)     | High DB and model load with little caching benefit          |
| 10 minutes (chosen)     | Balanced freshness and performance for moderate traffic     |
| Too long (> 1 hour)     | Risk of stale recommendations and reduced personalization   |

**Cache location:** Handled in the usecase layer (not handler or repository) because the usecase understands the full request boundary — it can skip all downstream DB and model calls on a cache hit.

**Cache injection:** `port.Cache` is injected into `UserUsecase` via the constructor (`NewUserUsecase`), not pulled from request context. This makes the cache dependency explicit, testable, and replaceable without changing business logic.

**Cache invalidation:** `POST /users/{id}/watch-history` records a new watch event and immediately calls `cache.DelPattern("rec:user:{id}:limit:*")`, which uses Redis `SCAN` to delete all cached recommendation sets for that user across all limit variants. This ensures the next recommendation request always reflects the latest watch history.

### Concurrency Control Approach

**Single endpoint:** No parallelism. A single request fetches one user's data sequentially — adding goroutines would increase complexity without benefit.

**Batch endpoint:** Bounded worker pool pattern.

```go
workers := min(limit, runtime.NumCPU() * 2)
```

- Prevents unbounded goroutine creation under high `limit` values
- Scales with available CPU cores
- Partial failures are isolated per-user — one failure does not cancel others
- Job distribution via buffered channels; results collected before response is built

### Error Handling Philosophy

Errors are categorized at the boundary where they originate and mapped to typed application errors:

| Category         | HTTP Status | Code                |
| ---------------- | ----------- | ------------------- |
| Invalid input    | 400         | `invalid_parameter` |
| User not found   | 404         | `user_not_found`    |
| Model failure    | 503         | `model_unavailable` |
| Request timeout  | 504         | `request_timeout`   |
| Unexpected error | 500         | `internal_error`    |

All errors propagate as `*apperr.Error` structs with a `Status`, `Code`, and `Message` field. The global error handler in `internal/adapter/http/middleware.go` converts these to consistent JSON:

```json
{"error": "user_not_found", "message": "User with ID 99 does not exist"}
```

Batch processing treats per-user model failures as non-fatal — they produce a `"status": "failed"` result rather than aborting the entire batch.

### Database Indexing Strategy

```sql
-- users table
CREATE INDEX idx_users_country ON users(country);
CREATE INDEX idx_users_subscription ON users(subscription_type);

-- content table
CREATE INDEX idx_content_genre ON content(genre);
CREATE INDEX idx_content_popularity ON content(popularity_score DESC);

-- user_watch_history table
CREATE INDEX idx_watch_history_user ON user_watch_history(user_id);
CREATE INDEX idx_watch_history_content ON user_watch_history(content_id);
CREATE INDEX idx_watch_history_composite ON user_watch_history(user_id, watched_at DESC);
```

**Rationale per index:**

| Index                                                    | Purpose                                                                     |
| -------------------------------------------------------- | --------------------------------------------------------------------------- |
| `idx_content_popularity DESC`                            | Supports `ORDER BY popularity_score DESC LIMIT 100` without full table scan |
| `idx_watch_history_composite (user_id, watched_at DESC)` | Covers watch history query sorted by recency — eliminates sort step         |
| `idx_watch_history_user`                                 | Fast lookup for N+1-avoidance query (`WHERE user_id = ANY($1)`)             |
| `idx_users_country`, `idx_users_subscription`            | Prepared for geo-filtering and subscription-tier filtering extensions       |

### Scoring Algorithm Rationale and Weight Choices

```go
// Step 1: Recency factor — exponential decay over 365 days
recencyFactor = 1.0 / (1.0 + daysSinceCreation / 365.0)
// Content from today ≈ 1.0, from 1 year ago ≈ 0.5

// Step 2: Genre preference — defaults to 0.1 if genre not in user history
genrePref = genrePreferences[content.Genre]  // 0.0–1.0 normalized frequency
if genrePref == 0 { genrePref = 0.1 }        // exploration floor

// Step 3: Final score
finalScore = (popularity_score × 0.40)   // dominant signal: universal appeal
           + (genrePref × 0.35)          // personalization: user taste
           + (recencyFactor × 0.15)      // recency: slight freshness boost
           + (uniform(-0.05, 0.05) × 0.1) // noise: ±0.5% exploration
```

**Weight rationale:**

| Component        | Weight | Reason                                                            |
| ---------------- | ------ | ----------------------------------------------------------------- |
| Popularity       | 40%    | Most reliable signal; works even for new users with no history    |
| Genre preference | 35%    | Core personalization driver derived from watch history            |
| Recency          | 15%    | Mild preference for newer content without overwhelming popularity |
| Random noise     | 10%    | Prevents identical results across calls; supports serendipity     |

The genre default of `0.1` (instead of `0`) ensures unvisited genres still receive a small exploration score rather than being permanently suppressed.

---

## 14.4 Performance Results

Performance testing was conducted using k6 at a constant arrival rate of **200 req/s for 2 minutes** per scenario. Results below are from actual test runs against the Dockerized stack (PostgreSQL 15 + Redis 7 + Go API) on a local machine.

---

### Cache Effectiveness Test

**Script:** `load/cache.js` — all requests hit `user_id=1` (fixed, always cached after first request)

**Configuration:** 200 req/s · 2 min · maxVUs 50

| Metric      | Result      | Threshold | Status |
| ----------- | ----------- | --------- | ------ |
| Avg Latency | **2.89 ms** | < 150 ms  | ✓      |
| P90 Latency | 4.01 ms     | —         | —      |
| P95 Latency | **5.50 ms** | < 200 ms  | ✓      |
| P99 Latency | **13.44 ms**| < 300 ms  | ✓      |
| Max Latency | 69.93 ms    | —         | —      |
| Error Rate  | **0.00%**   | < 1%      | ✓      |
| Throughput  | 200.09 req/s| —         | —      |
| Total Reqs  | 24,000      | —         | —      |

**Analysis:** The 2.89 ms average demonstrates that Redis cache hits effectively eliminate all DB and model overhead. P95 at 5.5 ms shows extremely stable latency under sustained load. Zero errors across 24,000 requests confirms system stability at this traffic level.

---

### Single User Load Test

**Script:** `load/single.js` — random `user_id` in range 1–2000

**Configuration:** 200 req/s · 2 min · maxVUs 300

| Metric      | Result       | Threshold | Status |
| ----------- | ------------ | --------- | ------ |
| Avg Latency | **7.00 ms**  | < 200 ms  | ✓      |
| P90 Latency | 9.98 ms      | —         | —      |
| P95 Latency | **43.91 ms** | < 300 ms  | ✓      |
| P99 Latency | **55.49 ms** | < 500 ms  | ✓      |
| Max Latency | 379.98 ms    | —         | —      |
| Error Rate  | **0.25%**    | < 1%      | ✓      |
| Throughput  | 199.91 req/s | —         | —      |
| Total Reqs  | 23,990       | —         | —      |

**Analysis:** The 7 ms average reflects a high cache hit ratio across 2,000 users — most requests return from Redis. The P95 jump to 43.91 ms corresponds to cache misses that trigger the full DB + model scoring path (30–50 ms simulated model delay). The 0.25% error rate is caused exclusively by the 1.5% random model failure simulation on cache-miss paths, which is within the defined 1% threshold at this hit ratio.

---

### Batch Endpoint Stress Test

**Script:** `load/batch.js` — `page=1&limit=20` (20 users scored per request)

**Configuration:** 200 req/s · 2 min · maxVUs 300

| Metric      | Result        | Threshold | Status |
| ----------- | ------------- | --------- | ------ |
| Avg Latency | **101.31 ms** | < 300 ms  | ✓      |
| P90 Latency | 107.16 ms     | —         | —      |
| P95 Latency | **168.16 ms** | < 500 ms  | ✓      |
| P99 Latency | **427.66 ms** | < 700 ms  | ✓      |
| Max Latency | 1,286.44 ms   | —         | —      |
| Error Rate  | **0.00%**     | < 1%      | ✓      |
| Throughput  | 198.98 req/s  | —         | —      |
| Total Reqs  | 23,896        | —         | —      |

**Analysis:** Higher latency is expected — each request concurrently scores 20 users, each with a 30–50 ms model delay, processed via a bounded worker pool. The 0% error rate confirms the worker pool prevents resource exhaustion even at 200 concurrent batch requests per second. Data throughput of 5.1 MB/s reflects the larger response payload (20 users × 10 recommendations each).

---

### Cache Hit Rate Analysis

Under random user ID distribution across 2,000 users at 200 req/s with a 10-minute TTL:

- The single endpoint averages **7 ms** (avg) vs **2.89 ms** (cache-only) — indicating the majority of requests are served from Redis
- The `cache_hit: true` field in every single-user response makes per-request hit/miss observable
- **Latency gap:** cache hit ~2.9 ms vs cache miss P95 ~43.9 ms — approximately **15× latency reduction** on cache hits

### Identified Bottlenecks

| Bottleneck                 | Observation                                                                    | Impact                                            |
| -------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------- |
| Model latency (30–50 ms)   | Dominant cost on cache miss; P95 latency closely follows model inference time  | Main contributor to cold-path latency             |
| Batch concurrent scoring   | ~20 users per batch processed in parallel (~40 ms model delay) via worker pool | Average batch request latency ~101 ms             |
| Batch max latency (1.28 s) | Occurs during VU spikes when worker pool queue builds up                       | P99 remains under 700 ms SLO                      |
| 500 ms single-user timeout | Limits number of DB + model round trips per request                            | Keeps latency within safe bound under normal load |

---

## 14.5 Trade-offs and Future Improvements

### Known Limitations

**Heuristic-based scoring**
The scoring function uses fixed weights (popularity 40%, genre 35%, recency 15%, noise 10%). It does not learn from user feedback, click-through rates, or implicit signals. Personalization depth is limited to genre frequency over the last 50 watched items.

**In-memory candidate scoring**
All candidates (up to 100 items) are loaded and scored in the service process. This is efficient at the current dataset size but would not scale for millions of content items.

**Single API instance**
The system runs as one container. There is no horizontal scaling, health-based routing, or load balancing configured.

### Scalability Considerations

- **API layer** can scale horizontally behind a load balancer. Redis is already external and shared, so cache state is consistent across instances.
- **PostgreSQL** can adopt read replicas for the heavy read workload (watch history, content queries). Table partitioning on `user_watch_history` by `user_id` range would improve large-scale read performance.
- **Batch processing** already uses bounded concurrency (`NumCPU × 2`), making it safe to run on multi-core instances. Increasing instance size directly improves batch throughput.
- **Cache warming** — pre-populating Redis for high-traffic users before peak load would reduce the cold-start latency spike.

### Proposed Enhancements If Time Permitted

| Enhancement                        | Description                                                                                                               |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Real ML model integration          | Replace heuristic scoring with a trained ranking model (e.g., two-tower retrieval + LTR re-ranking) served via gRPC       |
| Approximate Nearest Neighbor (ANN) | Replace linear candidate scoring with ANN index (e.g., pgvector or Faiss) for sub-millisecond retrieval at scale          |
| Observability                      | Integrate OpenTelemetry tracing, Prometheus metrics (cache hit rate, scoring latency histogram), and structured logging   |
| Unit and integration tests         | Add table-driven tests for scoring logic, cache key generation, and HTTP handler validation                               |
| Subscription-tier filtering        | Filter candidate content based on `subscription_type` (e.g., premium-only content hidden from free-tier users)            |
| Geographic content filtering       | Apply country-based availability rules during candidate selection                                                         |

