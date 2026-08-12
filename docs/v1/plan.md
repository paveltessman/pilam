# Implementation Plan

---

## Phases

### P0 — Toolchain and a running skeleton

Everything needed to type `make up` and see a page.

- `go.mod`, module path, Go version pinned.
- Tool versions pinned in `go.mod` via a tools file: `templ`, `sqlc`, `goose`, `golangci-lint`, `air`. The Tailwind standalone binary and the templUI CLI are fetched by a make target into `bin/`, gitignored — no Node in the toolchain, per `stack.md`.
- `Makefile`: `generate` (templ + sqlc), `css`, `build`, `run`, `test`, `lint`, `check`, `migrate`, `seed`, `up`, `down`.
- `docker compose`: Postgres, the app with live reload, a named volume for uploaded media, `.env.example` checked in and `.env` ignored.
- `cmd/pilam` with a `serve` subcommand, `/healthz`, and one templ page carrying a Tailwind class — enough to prove the generate → compile → serve → style pipeline end to end.

**Done when:** a fresh clone plus `make up` yields a styled page and a green `/healthz`, and `make check` passes.

---

### P1 — Platform services, the transport spine, and S1

The whole of `internal/platform`, `internal/http`'s skeleton, and the two subsystems that have no domain content: `auth` and `postgres`'s plumbing.

- **Platform:** `config` (typed, validated at boot, the only place `os.Getenv` appears, and the home of the business timezone), `logging`, `ids`, `validate`, `labels`, `media` (local-disk implementation), `session` (HMAC issue/verify).
- **`platform/date`:** helper functions over `time.Time`: `Of`, `Parse`, `Days(from, to)`, `Format`, comparisons. Values stay `time.Time` normalized to UTC midnight, so sqlc needs no type override and no pgx codec is written. What the package buys is that the operations that can be quietly wrong have exactly one implementation each. Pure, and dependency-free.
- **`platform/clock`:** `Now() time.Time` for instants — audit timestamps, session expiry — and `Today() time.Time` for the calendar day, in the business timezone baked in at construction from config.
- **Postgres plumbing:** pool, `InTx`, `sqlc` configuration, `migrate` subcommand with migrations embedded via `embed.FS`.
- **Transport:** `router.go`, the middleware chain (request id → logger → recover → session → identity → CSRF → HTMX detection), the base layout, static asset embedding, and the single error→status mapping the domain never touches.
- **`auth` + S1:** shared-login authentication, the `Identity` in context that the audit trail's actor comes from, and the login screen.
- **Lint rule:** `depguard` forbidding `pgx`, `net/http`, and `templ` inside domain packages.

**Done when:** logging in lands on an empty board and survives a refresh; a wrong password renders an inline field error through `validate`; `/healthz` round-trips the pool; a deliberately bad import in a domain package fails `make lint`; and `clock.Today` returns the configured business day for a fixed clock set to an hour that falls on a different date in UTC.

---

### P2 — The schema, in one pass

All migrations, all entities, written against `table_desc.md` and the decision ledger together.

- Catalog: season, capsule, drop, style, colorway, colorway photo.
- Calendar: milestone template, template item with its lead-time offset, milestone.
- Planning: plan version, channel quantity.
- Reference: fabric, partner and partner role, certificate, channel, person, and the controlled vocabularies of §3.5 as tables rather than Go constants — filters join against them, and O4's eventual admin screens become an addition rather than a rewrite.
- Audit: one table, written from the first domain write onward.
- Constraints that encode decisions: no destructive delete anywhere (soft `active` flags only), actual dates nullable and never defaulted (D10), baseline immutability enforced at the database where it can be.
- Calendar dates are `date` columns, never `timestamptz`. This is what actually holds the UTC-midnight invariant that `platform/date` assumes: pgx reads a `date` column as UTC midnight unconditionally.

**Done when:** `migrate up` and `down` are clean from empty, the resulting schema has been read against `table_desc.md` field by field, and `sqlc generate` produces compiling code.

---

### P3 — The pure kernel

No database, no HTTP, no clock. Table-driven tests only.

"No clock" is literal and it is what `platform/date` staying pure buys: anything here that needs the current day takes `today` as a parameter, and only the service layer above passes `clock.Today()` in. A function in this phase that reaches for the clock itself has stopped being testable at a fixed date, which is the entire point of writing it here.

- `calendar.Propagate(g Graph, changed MilestoneID, to time.Time) []Shift` — offset propagation down the critical path.
- At-risk derivation, and the two exception-list inclusion rules (forecast > baseline; baseline past and actual empty).
- Slack arithmetic for S7, including the sign convention: positive is early.
- The lead-time offsets — `жд-60`, `авто-45`, `автоTIR-30`, `авиа-14`, `+7` inbound, `−14` photo sample — as constants in the code.

**Done when:** the propagation and slack functions are exhaustively tested, including cycles, gaps, milestones with actuals already stamped, and off-calendar styles.

---

### P4 — `catalog`, `refdata`, and the seed's first half

The first phase with services, repositories, and audit writes.

- `catalog` and `refdata` services with their ports; `postgres` implementations; `audit.Recorder` wired and recording inside the same transaction as the write it describes.
- The `seed` subcommand: season SS27, capsules, three drops, reference data, ~60 styles, ~140 colorways, and generated placeholder imagery through the `media` interface. Deterministic via injected `ids` and a fixed RNG seed; all dates relative to `clock.Now()`.

**Done when:** `make seed` from an empty database produces the counts the PRD asks for, twice in a row byte-identically, and every seeded write has its audit row.

---

### P5 — `calendar` and `planning` persistence, and the seed's second half

- Milestone template instantiation on style creation, the three-date rules, and `MoveForecast` as a single transaction: load graph → `Propagate` → apply shifts → record audit.
- `planning`: DR/LR/ФАКТ versions with one flagged current, channel quantity rows, total derived by summation (D11).
- Seed extended: milestones for every style, plan versions for a subset, and the late distribution tuned until 8–12 styles are genuinely late, spread unevenly across the three drops, at least two alarmingly so.

**Done when:** a test asserts the seeded late distribution against the PRD's numbers, so a later seed change cannot quietly break steps 5 and 6.

---

### P6 — S2 season board and S3 style detail, read-only

- S2: card grid, group by drop with counts, filters (drop, status, at-risk, key item), free-text search across article and the three names.
- S3: header, design panel, sourcing and compliance panel, colorway strip, plan panel with its DR → LR → ФАКТ strip, collapsed details panel, milestone list in critical-path order with slip indicators.
- View models, Russian labels via `labels`, `DD.MM.YYYY` throughout, HTMX fragment routes as real named URLs.

**Done when:** demo step 1 runs — someone opens the board and recognises their own season.

---

### P7 — S3 editing, S4 colorway, media upload

Demo steps 2 and 3, and the answer to G1.

- Style-level editing with inline field errors; a trade-name change visibly propagating to every colorway.
- The colorway panel over S3: color code, `active` flag, multi-photo upload with the first as thumbnail; creation inheriting design-level facts structurally rather than by copying.
- Unticking `active` greys the card. Nothing deletes.

**Done when:** adding a fourth colorway takes about fifteen seconds and feels anticlimactic.

---

### P8 — S5 milestone drawer

Demo step 4, and the answer to G2.

- Drawer with name, type, owner, status, note; greyed baseline with a tooltip explaining why; forecast as the one editable date; "mark done" stamping today.
- Moving a forecast shifts downstream forecasts and flips the at-risk badge in the same response.
- Slip history — the audit trail's only surface — with who, when, old → new, and the summary line.

**Done when:** step 4's three visible consequences all happen from one date edit.

---

### P9 — S6 exception view

Demo step 5. `insight`'s own port and its own SQL — one indexed query, not per-entity repositories in a Go loop.

**Done when:** the style touched in step 4 appears near the top of the list, and each row clicks back into its milestone.

---

### P10 — S7 drop dashboard

Demo step 6.

- Per drop: target date, style count, projected on-time, late, mean slack.
- CSS bars, no charting library.
- Clicking a late segment opens S6 filtered to that drop.
- The line of on-screen copy stating what is being measured.

**Done when:** the numbers are reachable in two clicks from the board and reconcile by hand against the seed.

---
