## Setup Instructions

This section describes how to set up and run the system using the provided `Makefile` and `docker-compose.yml`.  
The project is fully containerized and requires no manual local dependency installation.

### Prerequisites
- Go 1.25
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+
- k6 (for performance testing)

### The Docker Compose file defines the following services:

    | Service  | Description              |
    | -------- | ------------------------ |
    | postgres | PostgreSQL 15 database   |
    | redis    | Redis 7 cache            |
    | migrate  | Runs database migrations |
    | seed     | Populates initial data   |
    | app      | Go API service           |

### Start All Services (Docker Compose)  
If your Makefile defines shortcuts:
``` make up ```
Or directly:
``` docker-compose up --build ```
This will:
- Start PostgreSQL
- Wait until database is healthy
- Run migrations
- Run seed process
- Start Redis
- Start API server

API will be available at:
``` http://localhost:8080 ```

### Running Migrations Manually

If you need to rerun migrations:
``` make migrate ```
Equivalent to:
``` docker-compose up migrate ```

Migration command executed:
    ```migrate -path=/migrations \-database=postgres://user:password@postgres:5432/recommendations?sslmode=disable \up```
    
### Running Seed Manually

To repopulate the database:
``` make seed ``` 
Equivalent to:
``` docker-compose up seed ``` 

### Running Individual Services
Start Only Database
``` make db ``` 
Equivalent to:
``` docker-compose up -d postgres ``` 
        
Start Only Redis
``` make redis ``` 
Equivalent to:
``` docker-compose up -d redis ``` 
        
Start Only API
``` make app ``` 
Equivalent to:
``` docker-compose up --build app ``` 
    
### Why This Setup Is Production-Oriented
- Healthcheck-based dependency control
- Isolated migration container
- Seed container separate from API
- Restart policy for API (unless-stopped)
- Persistent PostgreSQL volume
- Clear separation of concerns

This ensures reproducible, deterministic startup across environments.

### Architecture Overview
High-Level System Design

This system follows a Layered Architecture design, with clear separation between:
- Delivery layer (HTTP)
- Business logic layer (Usecase)
- Infrastructure adapters (Database, Cache, Model)
- Domain layer (Core entities)

### High-level architecture diagram:
                    ┌──────────────────────┐
                    │      Client (HTTP)   │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │      Handler Layer   │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │      Usecase Layer   │
                    └──────────┬───────────┘
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │ UserRepository  │  │   ModelClient   │  │ Cache (Redis)   │
    └────────┬────────┘  └────────┬────────┘  └────────┬────────┘
             ▼                    ▼                    ▼
    ┌──────────────┐     (Scoring Logic)      ┌──────────────┐
    │ PostgreSQL   │                          │    Redis     │
    └──────────────┘                          └──────────────┘

### Architectural Layers
##### Delivery Layer (Handler / HTTP)
- Location:
    ```
    internal/adapter/handler 
    internal/adapter/http
    ```
- Responsibilities:
    - Receive HTTP requests
    - Validate input parameters
    - Call usecase methods
    - Map application errors to HTTP responses
    - Serialize JSON output
    - The handler does not contain business logic.

- Usecase Layer (Business Logic)
    ```internal/usecase```
    This layer orchestrates the recommendation workflow.

- Domain Layer
        ```internal/domain```
    - Contains core business entities:
        - User
        - Content
        - WatchRecord
        - ScoredContent
        
    This layer contains no infrastructure code.
        It defines the data structures used across usecase and adapters

- Port Layer (Interface Contracts)
    ```internal/port```
    - Defines interfaces:
        - UserRepository
        - ModelClient
    - This allows:
        - Database implementation replacement
        - Model implementation abstraction
        - Easier testing and mocking

- Infrastructure Layer (Adapters)
    ```internal/adapter```
    - Contains implementations for:
        - PostgreSQL (adapter/pg + adapter/repository)
        - Redis cache (adapter/cache)
        - Model scoring logic (adapter/model)
        - These components integrate with external systems but contain no orchestration logic.

- Data Flow Through the System
    - Single Recommendation Flow
        1.Client calls:
                ```GET /users/{id}/recommendations?limit=10```
        2.Handler validates request
        3.Usecase performs cache lookup in Redis:
            - If hit → return cached response
            - If miss → continue
        4.Repository queries PostgreSQL for:
            - User data
            - Watch history
            - Candidate content
        5.Usecase builds user preference profile
        6.ModelClient scores candidate content
        7.Results are:
            - Sorted by score
            - Limited to requested size
        8.Response is cached (TTL: 10 minutes)
        9.JSON response returned
    - Batch Recommendation Flow
            1.Client calls:
                ```GET /recommendations/batch?page=1&limit=20```
            2.Usecase:
                - Fetches paginated user IDs
                - Bulk loads watch histories
                - Loads candidate content once
            3.Bounded worker pool processes users concurrently
            4.Each worker:
                - Builds preference
                - Calls ModelClient
                - Sorts results
            5.Aggregate success and failure
            6.Return structured batch response

    - How the Recommendation Model Integrates with Database Queries
        - Repository fetches structured domain data from PostgreSQL:
            - User profile
            - Watch history
            - Content metadata
        - Usecase prepares:
            - Genre preference map
            - Candidate content slice
    
        - ModelClient receives :
        ```
        ScoreCandidates(
            user domain.User,
            candidates []domain.Content,
            preference map[string]float64
        )
        ```
        - Model applies scoring algorithm:
            - Popularity weight (40%)
            - Genre match weight (35%)
            - Recency boost (15%)
            - Random noise (10%)
            - Simulated latency (30–50ms)
            - Simulated failure (1–2%)
        - Model returns scored results to usecase.
        - Usecase handles sorting and formatting.

### Design Decisions
##### Caching Strategy and TTL Rationale
###### Cache Strategy
The system uses a cache-first approach for the single recommendation endpoint.
            
Cache key format:
            ```rec:user:{user_id}:limit:{limit} ```
            
###### Design considerations:
- Include user_id to isolate personalization.
- Include limit to avoid incorrect reuse of differently-sized responses.
- Store full JSON response to avoid recomputation and re-sorting.
        
###### Why Cache at Usecase Level?
- Caching is handled during orchestration because:
    - The usecase understands request boundaries.
    - It avoids redundant model scoring.
    - It prevents unnecessary database reads.
- This approach reduces:
    - Database load
    - Model invocation frequency
    - CPU usage

###### TTL Rationale (10 minutes)
TTL is set to 10 minutes based on:
- Recommendation freshness requirements
    - Avoiding excessive recomputation
    - Reducing Redis memory footprint
-  Balancing staleness vs performance
-  Trade-off:
    - Short TTL → fresher results, higher load
    - Long TTL → better performance, possible staleness
For this dataset and traffic profile, 10 minutes provides a good balance

###### Concurrency Control Approach
Concurrency is applied in the batch endpoint only.

###### Why Not Parallelize Single Endpoint?
Single recommendation is already lightweight and cache-optimized.

###### Parallelization would introduce:
- Overhead
- Goroutine churn
- Increased complexity

###### Worker Pool Design (Batch Endpoint)
The batch endpoint uses a bounded worker pool:
    ```workers = min(limit, runtime.NumCPU() * 2)```
            
-   Rationale:
    -   Prevent unbounded goroutine creation
    -   Avoid CPU thrashing
    -   Maintain predictable memory usage
    -   Match workload to available CPU cores
-   Each worker:
    -   Processes one user at a time
    -   Applies per-user timeout
    -   Reports result to aggregation channel
        
-   Error Handling Philosophy
    -   Error handling is layered and categorized.
    -   Categories of Errors
        -   Validation errors (bad request)
        -   Not found errors (user does not exist)
        -   Model failure
        -   Timeout errors
        -   Infrastructure errors (DB / Redis)

###### Performance Results
Performance testing was conducted using k6 under sustained load for 2 minutes per scenario.
Each test used a constant arrival rate of 200 requests per second to simulate steady production traffic.
    
    Cache Endpoint Performance
        Test Configuration
            * 200 requests/sec
            * Duration: 2 minutes
            * Max VUs: 300
    
        | Metric      | Value     |
        | ----------- | --------- |
        | Avg Latency | ~4.64 ms  |
        | P95         | ~8.93 ms  |
        | P99         | ~30.83 ms |
        | Max         | ~511 ms   |
        | Error Rate  | ~0.12%    |

    Analysis
    The average latency of ~4 ms confirms that Redis cache hits significantly reduce response time by bypassing:
        Model scoring
        Database reads
        Sorting operations
    The P95 below 10 ms indicates stable latency distribution under sustained load.
    The small error rate (~0.12%) is within acceptable threshold (<1%) and primarily attributed to:
        Simulated model failure on rare cache misses
        Transient timeout conditions
    The system demonstrates strong I/O-bound performance in cache-hit scenarios.
    Single Recommendation Endpoint
        Test Configuration
            * 200 requests/sec
            * Duration: 2 minutes

        | Metric      | Value     |
        | ----------- | --------- |
        | Avg Latency | ~7.18 ms  |
        | P95         | ~45.31 ms |
        | P99         | ~55.66 ms |
        | Max         | ~250 ms   |
        | Error Rate  | ~0.14%    |

        Analysis
            The average latency (~7 ms) reflects a mix of:
                Cache hits (~4 ms)
                Model scoring path (~30–50 ms simulated)
            This indicates a high cache hit ratio.
            The latency spread (P95 ~45 ms) corresponds to requests that trigger model scoring.
            The system maintains consistent performance under sustained 200 rps without resource saturation.
    
    Batch Recommendation Endpoint
        Test Configuration
            * 200 requests/sec
            * Duration: 2 minutes
            * Bounded worker pool enabled

        | Metric      | Value      |
        | ----------- | ---------- |
        | Avg Latency | ~98.99 ms  |
        | P95         | ~171.43 ms |
        | P99         | ~379.62 ms |
        | Max         | ~903 ms    |
        | Error Rate  | 0%         |

        Analysis
            Batch endpoint latency is higher due to:
                Concurrent per-user scoring
                Worker pool orchestration
                In-memory sorting
                Aggregation of results
            Despite higher computational cost, the system maintained:
                0% error rate
                Stable latency distribution
                No N+1 database behavior
            This confirms proper concurrency control and bounded worker design.
        
   Performance Conclusion
        Under sustained 200 requests/sec:
            Cache endpoint is I/O-bound and extremely efficient
            Single endpoint benefits heavily from caching
            Batch endpoint is CPU-bound but remains stable
            No N+1 query behavior observed
            Error rate remains below defined threshold
            Worker pool prevents uncontrolled resource usage
        The system demonstrates production-ready performance characteristics and scalable architecture under moderate concurrent load.

#### Trade-offs and Future Improvements
    - Known Limitations
        -   Heuristic-Based Scoring
                - Current recommendation logic uses weighted rules (popularity, genre match, recency, randomness).
                -   It is not a real machine learning model, so personalization depth is limited.
        -   In-Memory Scoring
            - All candidate content is loaded and scored in memory.
            - This works well for moderate datasets but would not scale efficiently for millions of items.
        - TTL-Based Cache Only
            -   Cache invalidation relies on a fixed 10-minute TTL.
            -   User behavior changes do not immediately refresh recommendations.
        - Single API Instance
            - The system currently runs as a single API container.
            - Horizontal scaling and load balancing are not yet implemented.
    - Scalability Considerations
        API Layer can scale horizontally behind a load balancer.
        Redis already supports centralized caching for multi-instance deployments.
        PostgreSQL could use read replicas or partitioning for large datasets.
        Batch processing already uses bounded concurrency to prevent CPU overload.
Summary
    The current implementation prioritizes: 
        Simplicity
        Predictable performance
        Clean modular design
        Controlled concurrency
        While not fully production-scale for massive traffic, the architecture is extensible and ready to evolve into a distributed, ML-driven system.

## Final Summary

This recommendation service was designed with clarity, modularity, and performance in mind.

### The system demonstrates:

- Clean separation between delivery, business logic, and infrastructure
- Interface-driven design for extensibility and testability
- Cache-first optimization strategy
- Bounded concurrency for controlled CPU utilization
- Bulk-loading strategy to prevent N+1 query issues
- Stable performance under sustained load (200 req/sec)

### Performance testing confirms:

- Low latency in cache-hit scenarios (~4–5 ms average)
- Stable distribution under mixed workloads
- Controlled resource usage via worker pool
- Error rate consistently below defined thresholds

While the current implementation uses a heuristic scoring approach and runs as a single instance, the architecture is intentionally extensible. It can evolve toward:

- ML-driven ranking
- Horizontal scaling
- Distributed deployment
- Event-driven cache invalidation
- Advanced observability and monitoring

Overall, the system balances simplicity and production-readiness.  
It provides a solid architectural foundation that can scale and adapt to more advanced requirements.