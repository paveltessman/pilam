package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/paveltessman/pilam/internal/milestones"
)

var _ milestones.Atomic = (*DB)(nil)

const (
	milestoneTypeNameIndex = "milestone_type_name_key"
	milestoneTypeIDIndex   = "milestone_type_pkey"

	milestoneTemplateNameIndex    = "milestone_template_name_key"
	milestoneTemplateDefaultIndex = "milestone_template_default_key"
	milestoneTemplateIDIndex      = "milestone_template_pkey"

	milestoneItemTypeIndex  = "milestone_template_item_type_key"
	milestoneItemOrderIndex = "milestone_template_item_order_key"
	milestoneItemIDIndex    = "milestone_template_item_pkey"

	milestoneTypeOnModelIndex = "milestone_model_type_key"
	milestoneIDIndex          = "milestone_pkey"
)

// duplicateError translates the duplicate constraint to domain errors.
func duplicateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}

	switch pgErr.ConstraintName {
	case milestoneTypeNameIndex, milestoneTemplateNameIndex:
		return fmt.Errorf("%w: %w", milestones.ErrNameTaken, err)
	case milestoneItemTypeIndex:
		return fmt.Errorf("%w: %w", milestones.ErrTypeInTemplate, err)
	case milestoneTypeOnModelIndex:
		return fmt.Errorf("%w: %w", milestones.ErrTypeOnModel, err)
	case milestoneTypeIDIndex, milestoneTemplateIDIndex, milestoneItemIDIndex, milestoneIDIndex:
		return fmt.Errorf("%w: %w", milestones.ErrIDTaken, err)
	case milestoneTemplateDefaultIndex, milestoneItemOrderIndex:
		// The service holds both: it clears the default before it names a new
		// one, and it writes a whole order inside one transaction. Reaching
		// here is a bug in the service.
		return err
	default:
		// Reaching this means there is a new constraint in the db
		// that this function does not know about.
		panic(fmt.Sprintf("milestone repo: unknown constraint name: %s", pgErr.ConstraintName))
	}
}
