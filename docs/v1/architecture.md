# Backend Architecture v1

This document defines the code structure.

---

## 1. Layers and dependency direction

```
   cmd/pilam                composition root: wiring, lifecycle, shutdown
        │
        ▼
   internal/http            transport: router, middleware, handlers, views
        │                   knows HTTP and HTML. knows domain services.
        │                   knows nothing about SQL.
        ▼
   internal/<domain>        catalog · calendar · planning · refdata · insight
        │                   audit · auth
        │                   business rules and orchestration.
        │                   knows nothing about HTTP or SQL.
        ▼
   ports (interfaces declared inside each domain package)
        ▲
        │  implemented by
        │
   internal/postgres        sqlc queries, pgx pool, transactions
   internal/platform/*      clock · ids · media · session · logging · config
```
