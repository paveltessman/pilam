package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

var _ catalog.Atomic = (*DB)(nil)

// The names the catalog constraints carry, from 00004_catalog.sql.
const (
	seasonNameIndex = "season_name_key"
	seasonIDIndex   = "season_pkey"
	dropNameIndex   = "drop_season_name_key"
	dropIDIndex     = "drop_pkey"
	modelIDIndex    = "model_pkey"
	photoIDIndex    = "model_photo_pkey"
)

// identifier maps a filter identifier onto the argument. The zero identifier
// means the filter is off, and the statement reads that as NULL.
func identifier(id ids.ID) *ids.ID {
	if id == ids.Nil {
		return nil
	}
	return &id
}

// catalogError names the duplicate the write ran into.
func catalogError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}

	switch pgErr.ConstraintName {
	case seasonNameIndex:
		return fmt.Errorf("%w: %w", catalog.ErrSeasonNameTaken, err)
	case dropNameIndex:
		return fmt.Errorf("%w: %w", catalog.ErrDropNameTaken, err)
	case seasonIDIndex, dropIDIndex, modelIDIndex, photoIDIndex:
		return fmt.Errorf("%w: %w", catalog.ErrIDTaken, err)
	}
	return err
}
