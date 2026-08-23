package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/paveltessman/pilam/internal/milestone"
)

var _ milestone.Atomic = (*DB)(nil)

const (
	milestoneTypeNameIndex = "milestone_type_name_key"
	milestoneTypeIDIndex   = "milestone_type_pkey"
)

// duplicateError translates the duplicate constraint to domain errors.
func duplicateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}

	switch pgErr.ConstraintName {
	case milestoneTypeNameIndex:
		return fmt.Errorf("%w: %w", milestone.ErrNameTaken, err)
	case milestoneTypeIDIndex:
		return fmt.Errorf("%w: %w", milestone.ErrIDTaken, err)
	default:
		// Reaching this means there is a new constraint in the db
		// that this function does not know about.
		panic(fmt.Sprintf("milestone repo: unknown constraint name: %s", pgErr.ConstraintName))
	}
}
