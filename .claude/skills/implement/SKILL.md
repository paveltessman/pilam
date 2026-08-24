---
name: implement
description: Build a feature in reviewed steps. Plan first, split the work by commits, then write one commit at a time and stop for review. Use when the user asks to implement a feature, a PR of a plan document, or a change described in text, and wants to review each step before it lands.
---

# implement

Three phases. The user reviews between every one of them. You never commit.

```
1. Plan      you read, you ask, you propose commits   -> user approves
2. Build     you write one commit, you verify, stop   -> user reviews and commits
3. Resume    you read HEAD, you build the next one    -> repeat until done
```

## Phase 1: plan

The user names the work as text, or points at a document such as `docs/v1/<feature>/plan.md`.

Read before you plan:

- The document the user named, and the design document above it.
- The code the change touches. Read whole files, not excerpts.
- The nearest thing that already works. A new screen follows the screens that exist. A new store follows the stores that exist.
- The tests of that nearest thing. They state the conventions.

Then write the plan in the chat. It holds:

1. **What the change adds**, in four or five lines. Name the layers it touches.
2. **The decisions you made**, with the reason. A design document leaves gaps. Fill them, state that you filled them, and let the user correct you before any code exists.
3. **The commits.** One heading each, with the files and the tests. Say what makes each one green on its own.
4. **What stays out**, and why. Quote the plan document where it defers work.

Split the commits along the dependency order, so each one builds and passes on its own:

| Order | Commit                                                 |
| ----- | ------------------------------------------------------ |
| 1     | Pure domain: types, rules, ports. Wired to nothing.    |
| 2     | The migration, the queries, the store that fills them. |
| 3     | The service writes, and the wiring they need.          |
| 4     | Cross-cutting plumbing the screen will want.           |
| 5     | The screen, read only.                                 |
| 6     | The screen actions: routes, forms, handlers.           |

Not every change needs six. Merge what is thin, split what is fat. A commit that reads as two paragraphs of review is the right size.

Ask a question only where two readings lead to different work. Anything else is a decision you state in the plan.

**Stop. Wait for the user to approve the plan.**

## Phase 2: build one commit

Work the current commit and nothing else. Do not start the next one.

Before you stop, every one of these passes:

- `make check` runs tidy, vet, tests and the linter. Run it, or run its parts.
- Regenerate what is generated: `go tool templ generate`, `go tool sqlc generate`, `make css`. Generated files are ignored by git, so the build regenerates them, but a broken one fails your own tests.
- `gofmt -l` on every directory you touched.

Then report in the chat:

- **What to look at**: a decision worth a second opinion, a trade-off, an assumption.
- **What you had to change outside the commit.** A later commit that reworks a screen breaks the tests of an earlier one. Fix them and say so.

**Stop. Do not run `git commit`. The user commits.**

## Phase 3: resume

The user commits by hand, and changes things while doing it. Before the next commit:

- Read `git log --oneline -3` and `git show --stat HEAD`.
- Check the names you introduced last time. A constant, a label, or a function may carry a new name now.
- Take the new name as the current state. Never silently revert it. If the change is bad, stop and tell the user about it.

## Rules

- **Never commit, never push.** The user does both.
- **One commit per turn.** Green at the end of each.
- **Do not narrow the scope.** If part of the commit is blocked, finish the rest in full and say what you left out.
- **Match the code around you.** Comment density, naming, error wording, test style. The nearest working file is the specification.
- **Write the comments and the chat in ASD-STE100.** The user rule set applies. Code and identifiers do not.
- **Say when a test is weak.** A screen test that passes on text from elsewhere on the page proves nothing. Scope it to the part under test.

## This project

- Layers: `cmd` -> `internal/http` -> `internal/<domain>` -> ports, filled by `internal/postgres` and `internal/platform`. A domain package holds no HTTP, no SQL, no templ. The linter refuses it.
- Every write lands in the audit trail, inside the transaction that carries it.
- Screens live under `internal/http/<section>`, views under `<section>/views`, routes in `internal/http/paths` and `internal/http/router.go`.
- User-visible text lives in `internal/platform/labels`. No literal text in a view.
- Postgres tests need a real database and `TEST_DATABASE_URL` and run in parallel, one schema per test.
- The screen tests build their fixtures through `internal/http/testkit`.
