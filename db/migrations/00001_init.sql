-- This migration is deliberately empty:
-- it exists so that the pipeline — embed, goose, up, down — is provable
-- before there is anything to migrate, and so `//go:embed migrations/*.sql` has
-- a file to match.
--
-- goose records it as an applied version and reports it as EMPTY. Do not add
-- statements here; add the next migration instead.

-- +goose Up

-- +goose Down
