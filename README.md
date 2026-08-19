# pilam

A PLM for a fashion brand's seasonal calendar. See [`docs/v1`](docs/v1).

## Run

```
make up      # Postgres + the app with live reload, on http://localhost:8080
make down    # stop, keeping the database and uploaded media
make logs    # follow the app
```

`make up` copies `.env.example` to `.env` on first run and needs nothing installed beyond Docker. Everything else — Go, Tailwind, templ — lives in the container.

To run on the host instead, with Go 1.26 installed:

```
make tools   # fetch the Tailwind CLI and the templUI CLI into ./bin (~110MB, once)
make run     # http://localhost:8080
```

## Work on it

```
make check   # what CI runs: generate, css, vet, test, lint
make help    # every target
```

Database schema:

```
make migrate                        # apply pending migrations
make migrate-status                 # what is applied, what is pending
make migration name=add_styles      # scaffold the next one
```

Migrations are embedded in the binary.

The first account:

```
make user email=a@b.c name="Ada Lovelace" root=1
```

The command generates the password and prints it once. Copy it before you close the terminal: nothing stores it, and the new user must change it at the first login. Leave `root=1` out to create a member.

The target reaches Postgres on the published port. To run it inside the dev container instead:

```
docker compose exec app go run ./cmd/pilam user add --email a@b.c --name "Ada Lovelace" --root
```

Demo data:

```
make seed                                  # the whole roster
make seed users=5 domain=brand.example     # a shorter list, on another domain
```

`pilam seed` loads the demo dataset. Today the dataset is the company: 24 employees, two of them root and two deactivated. Every seeded user logs in with the one password the command prints, and lands on the board without changing it first. A second run over the same database writes nothing and reports the rows it skipped.

The tests that need a real Postgres — the transaction runner and the migrations — skip unless `TEST_DATABASE_URL` is set. With the stack up:

```
TEST_DATABASE_URL=postgres://pilam:pilam@localhost:5433/pilam?sslmode=disable make test
```

`make test` and `make check` run the suite through `gotestsum`.

Two kinds of file are generated and gitignored: `*_templ.go` (from `.templ` sources, via `make generate`) and `internal/http/static/css/app.css` (via `make css`). Both are rebuilt by any target that needs them, so a fresh clone only ever needs a `make` target, never a manual step.

## Toolchain

Go tools — `templ`, `sqlc`, `goose`, `golangci-lint`, `air`, `gotestsum` — are pinned as `tool` directives in `go.mod` and run through `go tool`. The two that are not Go modules, the Tailwind standalone CLI and the templUI CLI, are fetched into `./bin` by `make tools`.

templUI components are vendored into `internal/http/views/ui/` by the templUI CLI (`bin/templui add <component>`) rather than imported. They are third-party source: update them with the CLI, do not edit them by hand. The linter is configured to skip them.
