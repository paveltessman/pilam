package milestones

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// Type is a milestone type, used in milestone templates.
type Type struct {
	ID          ids.ID
	Name        string
	Description string
	Active      bool
}

// TypesStore is the store of milestone types.
// Writes return ErrNameTaken on a duplicate name.
type TypesStore interface {
	// ByID returns ErrNoType when nothing matches
	ByID(ctx context.Context, id ids.ID) (Type, error)

	// List returns every type, active and inactive, ordered by short name.
	List(ctx context.Context) ([]Type, error)

	Create(ctx context.Context, in Type) error
	Update(ctx context.Context, in Type) error
}

type TypeCreateParams struct {
	Name        string
	Description string
}

type TypeUpdateParams struct {
	Name        string
	Description string
	Active      bool
}

// Type returns one milestone type.
func (s *Service) Type(ctx context.Context, typeID ids.ID) (Type, error) {
	t, err := s.types.ByID(ctx, typeID)
	if err != nil {
		return Type{}, fmt.Errorf("milestone: loading type %s: %w", typeID, err)
	}
	return t, nil
}

// ListTypes returns every milestone type, active and inactive, by short name.
func (s *Service) ListTypes(ctx context.Context) ([]Type, error) {
	types, err := s.types.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing types: %w", err)
	}
	return types, nil
}

// CreateType writes a new milestone type and returns the row it wrote.
func (s *Service) CreateType(ctx context.Context, in TypeCreateParams) (Type, error) {
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)

	if err := checkNaming(name, description); err != nil {
		return Type{}, err
	}

	milestoneType := Type{
		ID:          s.ids.New(),
		Name:        name,
		Description: description,
		Active:      true,
	}

	write := func(ctx context.Context) error {
		if err := s.types.Create(ctx, milestoneType); err != nil {
			return err
		}
		err := s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneType,
			EntityID: milestoneType.ID,
			Action:   audit.ActionCreated,
			FieldKey: "",
			Old:      "",
			New:      milestoneType.Name,
		},
		)
		return err
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Type{}, err
	}
	return milestoneType, nil
}

// UpdateType writes the edited fields and records one trail entry per field
// that moved. It writes nothing when nothing moved.
func (s *Service) UpdateType(ctx context.Context, typeID ids.ID, in TypeUpdateParams) error {
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)

	if err := checkNaming(name, description); err != nil {
		return err
	}

	milestoneType, err := s.types.ByID(ctx, typeID)
	if err != nil {
		return fmt.Errorf("milestone: loading type %s: %w", typeID, err)
	}

	var changes []audit.Change
	if name != milestoneType.Name {
		changes = append(changes, audit.Change{
			Entity:   audit.EntityMilestoneType,
			EntityID: milestoneType.ID,
			Action:   audit.ActionChanged,
			FieldKey: FieldName,
			Old:      milestoneType.Name,
			New:      name,
		})
		milestoneType.Name = name
	}
	if description != milestoneType.Description {
		changes = append(changes, audit.Change{
			Entity:   audit.EntityMilestoneType,
			EntityID: milestoneType.ID,
			Action:   audit.ActionChanged,
			FieldKey: FieldDescription,
			Old:      milestoneType.Description,
			New:      description,
		})
		milestoneType.Description = description
	}
	if in.Active != milestoneType.Active {
		changes = append(changes, audit.Change{
			Entity:   audit.EntityMilestoneType,
			EntityID: milestoneType.ID,
			Action:   audit.Activation(in.Active),
			FieldKey: FieldActive,
			Old:      strconv.FormatBool(milestoneType.Active),
			New:      strconv.FormatBool(in.Active),
		})
		milestoneType.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	err = s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.types.Update(ctx, milestoneType); err != nil {
			return err
		}
		return s.RecordTrail(ctx, changes...)
	})
	return err
}
