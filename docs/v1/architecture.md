# Backend Architecture — v1

This document defines the code structure: what the layers are, what services exist, which way dependencies point.

---

## 1. Principles

**P1 — Abstract by capability, not by storage primitive.**
Every subsystem is reached through an interface expressed in the language of the domain: `catalog.Styles.ListForSeason(ctx, filter)`, `media.Store.Put(ctx, upload)`, `audit.Recorder.Record(ctx, entry)`. No caller outside the implementing package ever sees `pgx`, `pgtype`, a table name, a file path, or an HTTP status code from a layer that shouldn't own one.

**P2 — Interfaces are defined by the consumer.**
`catalog` declares the storage interface it needs, in `catalog`. `postgres` implements it. Dependencies point inward; the domain imports nothing from `http/` or `pgx`.

**P3 — Business rules are pure functions where they can be.**
Lead-time offset propagation, at-risk derivation, OTIF slack — no I/O in any of them. Services orchestrate; arithmetic is testable with no database and no clock.

**P4 — One composition root.**
`cmd/pilam/main.go` is the only place concrete types are constructed and wired. No service locator, no DI container, no `init()` registration.

**P5 — Everything ambient is injected.**
Time, ID generation, and randomness are services. This project is entirely about dates; `time.Now()` scattered through the domain makes every KPI untestable and the seed non-reproducible.

**P6 — Don't abstract what you won't replace.**
There is no interface over templ, over `net/http`, or over the router. See §10.

---

## 2. Layers and dependency direction

```
   cmd/pilam                composition root — wiring, lifecycle, shutdown
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

The rule in one line: **an import of `pgx`, `net/http`, or `templ` inside a domain package is a bug.** To be inforced with a lint rule.

---

## 3. Package layout

```
cmd/
  pilam/
    main.go              wiring, lifecycle
    serve.go             http server + graceful shutdown
    migrate.go           goose up/down subcommand
    seed.go              demo seed subcommand

internal/
  platform/
    config/              typed config, validated at boot
    clock/               Clock interface + real + fixed
    ids/                 ID generation
    logging/             slog setup, request-scoped logger
    media/               blob storage: interface + implementations (local, s3, ...)
    session/             signed cookie issue/verify (HMAC)
    validate/            field validation + error shape for inline form errors
    labels/              Russian UI vocabulary, date/number formatting

  catalog/               Season · Capsule · Drop · Style · Colorway
  calendar/              Milestones, templates, offsets, at-risk  (D9/D10)
  planning/              Plan versions, channel quantities        (D15/D11)
  refdata/               Fabric · Partner · Certificate · Channel · People · vocabularies
  insight/               Exception list + drop dashboard read models
  audit/                 Change history (D17)
  auth/                  Identity, shared-login authentication    (D16)

  postgres/              pgx pool, tx runner, sqlc output, all repo implementations
    gen/                 sqlc-generated code (never imported outside this package)

  seed/                  demo dataset generator (S0)

  http/
    router.go
    middleware/
    handlers/            one package per screen: board, style, colorway,
                         milestone, exceptions, drops, session
    views/               templ components + view models
    static/              embedded css/js

db/
  migrations/            goose, plain SQL, embedded
  queries/               sqlc source SQL
```

**Why all repository implementations live in one `postgres` package:** they share the sqlc `Queries` type and, more importantly, a transaction. `calendar.MoveForecast` writes milestones *and* an audit entry in one transaction; if those implementations live in separate packages, you need an exported transaction type crossing package boundaries and the seam turns into plumbing. One package, many files (`postgres/styles.go`, `postgres/milestones.go`, `postgres/audit.go`), each implementing one domain's port.

---

## 4. Platform services

Domain-ignorant, infrastructure-facing. Each is an interface with exactly one production implementation and a trivial test double.

| Service | Description |
|---|---|---|
| `config` | Typed config from env, validated once at boot. No `os.Getenv` anywhere else. Misconfiguration fails at startup. |
| `logging` | slog handler, request-scoped logger with request id. One place to change format/level. |
| `clock` | `Now() time.Time`. Injectable time is what makes this all testable and the seed reproducible. |
| `ids` | ID generation. Deterministic variant lets the seed produce identical output across runs, so demo bookmarks and screenshots stay valid. |
| `media` | Colorway photos: `Put`, `Open`, `URL`, size/type validation |
| `session` | Signed cookie issue/verify. Isolates the HMAC handling to one auditable file. |
| `validate` | Field validation, structured field errors. HTMX renders errors inline per field; the error shape has to be uniform or every form is bespoke. |
| `labels` | Russian UI strings, `DD.MM.YYYY` dates, ₽/¥ formatting. PRD cross-cutting rule: UI labels in Russian using the sheet's verbatim vocabulary, code in English. Concentrating that mapping keeps Russian string literals out of both domain and templ files, and makes the 15-status vocabulary a single table. |

---

## 5. Domain services

Each owns a slice of the model, enforces its own invariants, and exposes use-case methods.

### `catalog` — Season, Capsule, Drop, Style, Colorway

Owns the D18 split. The invariant that makes the demo's step 2 work — design-level facts are stored once and colorways inherit them structurally, not by copying — lives here. Also owns colorway creation (step 3), the `active` soft-delete flag, and photo attachment via `media`.

Notable: **at-risk is derived, not stored.** With ~60 styles there is no reason to maintain a denormalized flag that can go stale; `insight` computes it. If it ever needs an index, it becomes a generated column, not a field.

### `calendar` — the critical path (D9, D10)

Owns:

- the hardcoded milestone template and its instantiation on style creation;
- the three dates and their rules — baseline immutable, forecast editable, actual only ever stamped, never seeded (D10);
- offset propagation: moving a forecast shifts downstream forecasts using lead-time offsets (the `жд-60`/`авто-45`/`автоTIR-30`/`авиа-14` transit constants, `+7` inbound, `−14` photo sample). These are configuration data, not constants in the code.

The propagation itself is a pure function:

```go
// no I/O, no clock, no database
func Propagate(g Graph, changed MilestoneID, to date.Date) []Shift
```

That function is the highest-risk logic in the product and it should be unit-tested to death without a database in sight.

### `planning` — plan versions (D15, D11, D2)

DR / LR / ФАКТ as versions of one entity, one flagged current. Channel quantities as rows, with the total **derived** by summation — which is precisely the integrity bug visible in the source's mock row (530 + 50 ≠ 594).

### `refdata` — Fabric, Partner, Certificate, Channel, People, vocabularies

Read-only in v1 (D20/D21 are schema-only), seeded, reached through pickers. It exists as a real service anyway so that the eventual admin CRUD is an addition to one package and callers never move. Also the home of the controlled vocabularies from `table_desc.md` §3.5 and the 15-state status list.

### `insight` — the two views that justify the product (S6, S7)

Deliberately separate from `catalog` and `calendar`. These are cross-entity read models with their own queries and their own shapes:

- **Exception list**: every milestone in the season where `forecast > baseline`, or `baseline < today AND actual IS NULL`, sorted by slip.
- **Drop dashboard**: per drop, projected on-time / late counts and mean slack, where slack = drop target date − (actual arrival if present, else current forecast).

Read-only, no writes, its own port, its own SQL. Do not route these through per-entity repositories — that turns one indexed query into an N+1 and buries the business rule in Go loops. The slack arithmetic and the sign convention (positive = early, matching `CX`) stay as pure functions in the domain; the aggregation is SQL.

### `audit` — change history (D17)

Every write records who, what, when, old → new.

Recorded explicitly by the service performing the write, inside the same transaction. Not a database trigger. Triggers cannot see the actor without session variables.

### `auth` — identity (D16)

One shared login, one role that sees and edits everything. Shaped so RBAC can arrive later: `Authenticate(ctx, creds) (Identity, error)`, and an `Identity` carried in context that already has a place for a role. v1's implementation compares against a configured credential and returns the single manager identity. The audit trail's actor comes from here, which is the reason identity exists at all in a single-user demo.

### `seed` — the demo dataset (S0)

A service and a CLI subcommand, not a `.sql` file. The seed has requirements that SQL literals cannot meet: dates must be relative to `clock.Now()` so the demo reads as "mid-development" whenever it runs, and it must produce 8–12 genuinely late styles spread unevenly across three drops, at least two alarmingly late. That is generation logic with business meaning, and it deserves the same services and the same tests as production code. Deterministic via injected `ids` and a fixed RNG seed.

---

## 6. Transport layer

**`router.go`** — the whole route table in one file, readable top to bottom.

**`middleware/`** — request id, structured logging, panic recovery, session loading, auth guard, CSRF, HTMX request detection.

**`handlers/`** — one package per PRD screen (`board`, `style`, `colorway`, `milestone`, `exceptions`, `drops`, `session`). Every handler does exactly four things:

1. Parse and validate input.
2. call **one** domain service method. A handler that calls three services and coordinates them is a use case and should be moved into the domain.
3. Map the result to a view model.
4. Render — full page, or fragment when `HX-Request` is set.


**`views/`** — templ components plus the view models they consume. View models are plain structs owned by the view layer. Domain types are not passed to templates directly: it is what keeps Russian labels, `DD.MM.YYYY` formatting, and "`+12д`" badge strings out of the domain, and what stops a template's needs from deforming a domain type.

**HTMX fragment convention.** Every partial-rendering endpoint is a real route with a real URL, named for what it returns (`GET /styles/{id}/milestones`), not a flag on another route. Full-page and fragment responses render the *same* component, wrapped in the layout or not. One decision point, in one middleware.

---

## 7. Cross-cutting conventions

**Transactions.** A use case that must be atomic wraps itself: `db.InTx(ctx, func(tx Tx) error { ... })`. The transaction is passed explicitly, never smuggled through `context.Value`. Handlers never open transactions — if a handler needs one, the use case is in the wrong layer.

**Errors.** Domain packages return typed errors carrying a code and a user-facing message (`catalog.ErrNotFound`, `validate.FieldErrors`). The HTTP layer owns the entire mapping from error to status code and rendered message. No domain package imports `net/http` to say "404".

**Time.** `clock.Now()` is always used. Dates are calendar dates, not timestamps.

**Validation.** One vocabulary of field errors from `platform/validate`, rendered inline by a shared templ component. Domain invariants (baseline is immutable) are enforced in the domain and surface as the same error type, so the form doesn't care where the rejection came from.

---

## 8. Worked trace: demo step 4

"The fabric is two weeks late" — the path through every layer, as a consistency check:

```
POST /milestones/{id}/forecast        (HTMX, from the S5 drawer)
  │
  ├─ middleware: request id → logger → session → identity → CSRF
  │
  ├─ handlers/milestone.UpdateForecast
  │     parse date, validate, call service method
  │
  ├─ calendar.Service.MoveForecast(ctx, id, newDate)
  │     db.InTx:
  │       ├─ milestones.Get + LoadGraph(styleID)
  │       ├─ calendar.Propagate(graph, id, newDate)   ← pure, no I/O
  │       ├─ milestones.ApplyShifts(shifts)
  │       └─ audit.Record(actor, "milestone.forecast_moved", old→new)
  │
  ├─ handlers map result → view model (Russian labels, DD.MM.YYYY, "+12д")
  │
  └─ render fragment: milestone list + at-risk badge
        at-risk recomputed by insight, not read from a stored flag
```

---

## 9. Mapping: screens → services

| Screen | Primary service | Also uses |
|---|---|---|
| S1 Login | `auth` | `session` |
| S2 Season board | `catalog` | `insight` (at-risk), `refdata` (filter vocabularies) |
| S3 Style detail | `catalog` | `calendar`, `planning`, `refdata`, `insight` |
| S4 Colorway | `catalog` | `media` |
| S5 Milestone drawer | `calendar` | `audit` (slip history), `refdata` (owners) |
| S6 Exception view | `insight` | `refdata` (filters) |
| S7 Drop dashboard | `insight` | `catalog` (drops) |

---

## 10. Deliberately not abstracted

Applying P1 everywhere would cost more than it returns. These stay concrete:

- **`net/http` and the router.** Handlers take `(w, r)`. No framework-agnostic transport layer — there is one transport and it is HTTP.
- **templ.** It is already the abstraction; wrapping typed compiled components in an interface buys nothing.
- **A generic `Repository[T]`.** Domain-shaped methods or nothing. A generic repository is a table gateway with extra steps, and it makes the two read-model screens impossible to express.
- **An event bus.** v1 has one in-process actor and one transaction per use case. Direct calls. Revisit if and when milestone changes need to notify anything outside the process.
- **A DI container.** `main.go` wires by hand. When wiring becomes painful, that is a signal about coupling, and a container would only hide it.
- **A CQRS split.** `insight` having its own read-side queries is the useful 10% of CQRS; the rest is ceremony at this scale.

---
