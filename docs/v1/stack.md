# Tech Stack

| Layer | Choice |
|---|---|
| Language | **Go** |
| Database | **PostgreSQL** |
| HTTP routing | stdlib `net/http` |
| Templating | **templ** — typed components compiled to Go |
| Interactivity | **HTMX**, plus **Alpine.js** for local-only UI state |
| CSS | **Tailwind**, standalone CLI (no Node in the toolchain) |
| Components | **templUI** |
| DB driver | **pgx v5** |
| Query layer | **sqlc** — hand-written SQL, generated typed Go |
| Migrations | **goose** — plain SQL, embedded via `embed.FS` |
| Session | Hand-rolled signed cookie (`crypto/hmac`) |
