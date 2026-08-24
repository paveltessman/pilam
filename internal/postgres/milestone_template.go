package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var _ milestones.TemplatesStore = (*MilestoneTemplates)(nil)

// MilestoneTemplates is the store of templates and of the items they hold. The
// item is part of the template, so one store carries both.
type MilestoneTemplates struct {
	db *DB
}

func NewMilestoneTemplates(db *DB) *MilestoneTemplates {
	if db == nil {
		panic("postgres: nil database")
	}
	return &MilestoneTemplates{db: db}
}

// ByID returns the template, or milestones.ErrNoTemplate when there is no such
// row.
func (m *MilestoneTemplates) ByID(ctx context.Context, id ids.ID) (milestones.Template, error) {
	row, err := m.db.queries(ctx).GetMilestoneTemplate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return milestones.Template{}, fmt.Errorf("%w: id %s", milestones.ErrNoTemplate, id)
		}
		return milestones.Template{}, fmt.Errorf("postgres: loading milestone template %s: %w", id, err)
	}
	return milestoneTemplate(row), nil
}

// List returns every template, active and inactive, ordered by name.
func (m *MilestoneTemplates) List(ctx context.Context) ([]milestones.Template, error) {
	rows, err := m.db.queries(ctx).ListMilestoneTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing milestone templates: %w", err)
	}

	templates := make([]milestones.Template, len(rows))
	for i, row := range rows {
		templates[i] = milestoneTemplate(row)
	}
	return templates, nil
}

// Create writes one new row. A taken name returns milestones.ErrNameTaken.
func (m *MilestoneTemplates) Create(ctx context.Context, in milestones.Template) error {
	params := sqlc.CreateMilestoneTemplateParams{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		IsDefault:   in.Default,
		Active:      in.Active,
	}

	if err := m.db.queries(ctx).CreateMilestoneTemplate(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating milestone template %s: %w", in.ID, duplicateError(err))
	}
	return nil
}

// Update writes every field back to the row.
func (m *MilestoneTemplates) Update(ctx context.Context, in milestones.Template) error {
	params := sqlc.UpdateMilestoneTemplateParams{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		IsDefault:   in.Default,
		Active:      in.Active,
	}

	written, err := m.db.queries(ctx).UpdateMilestoneTemplate(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating milestone template %s: %w", in.ID, duplicateError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", milestones.ErrNoTemplate, in.ID)
	}
	return nil
}

// ClearDefault drops the default flag of every template but the one named.
func (m *MilestoneTemplates) ClearDefault(ctx context.Context, keep ids.ID) error {
	if err := m.db.queries(ctx).ClearMilestoneTemplateDefault(ctx, keep); err != nil {
		return fmt.Errorf("postgres: clearing the default template around %s: %w", keep, err)
	}
	return nil
}

// Items returns the items of one template, in position order.
func (m *MilestoneTemplates) Items(ctx context.Context, templateID ids.ID) ([]milestones.TemplateItem, error) {
	rows, err := m.db.queries(ctx).ListMilestoneTemplateItems(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing the items of milestone template %s: %w", templateID, err)
	}

	items := make([]milestones.TemplateItem, len(rows))
	for i, row := range rows {
		items[i] = milestoneTemplateItem(row)
	}
	return items, nil
}

// AddItem writes one new item. A type the template already holds returns
// milestones.ErrTypeInTemplate.
func (m *MilestoneTemplates) AddItem(ctx context.Context, in milestones.TemplateItem) error {
	params := sqlc.CreateMilestoneTemplateItemParams{
		ID:         in.ID,
		TemplateID: in.TemplateID,
		TypeID:     in.TypeID,
		OffsetDays: int32(in.Offset),
		Position:   int32(in.Position),
	}

	if err := m.db.queries(ctx).CreateMilestoneTemplateItem(ctx, params); err != nil {
		return fmt.Errorf("postgres: adding item %s to milestone template %s: %w",
			in.ID, in.TemplateID, duplicateError(err))
	}
	return nil
}

// UpdateItem writes every field back to the row.
func (m *MilestoneTemplates) UpdateItem(ctx context.Context, in milestones.TemplateItem) error {
	params := sqlc.UpdateMilestoneTemplateItemParams{
		ID:         in.ID,
		TypeID:     in.TypeID,
		OffsetDays: int32(in.Offset),
		Position:   int32(in.Position),
	}

	written, err := m.db.queries(ctx).UpdateMilestoneTemplateItem(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating item %s of milestone template %s: %w",
			in.ID, in.TemplateID, duplicateError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", milestones.ErrNoItem, in.ID)
	}
	return nil
}

// RemoveItem deletes one row, and milestones.ErrNoItem when there is nothing to
// delete.
func (m *MilestoneTemplates) RemoveItem(ctx context.Context, itemID ids.ID) error {
	written, err := m.db.queries(ctx).DeleteMilestoneTemplateItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("postgres: removing item %s: %w", itemID, err)
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", milestones.ErrNoItem, itemID)
	}
	return nil
}

// ReorderItems writes position i to the i-th identifier of ordered.
//
// The order key of the table is deferred, so the rows hold the same position
// for a moment on the way and the index reads them at the commit.
func (m *MilestoneTemplates) ReorderItems(ctx context.Context, ordered []ids.ID) error {
	if len(ordered) == 0 {
		return nil
	}

	write := func(ctx context.Context) error {
		queries := m.db.queries(ctx)
		for position, itemID := range ordered {
			params := sqlc.SetMilestoneTemplateItemPositionParams{ID: itemID, Position: int32(position)}
			written, err := queries.SetMilestoneTemplateItemPosition(ctx, params)
			if err != nil {
				return fmt.Errorf("postgres: moving item %s to position %d: %w", itemID, position, err)
			}
			if written == 0 {
				return fmt.Errorf("%w: id %s", milestones.ErrNoItem, itemID)
			}
		}
		return nil
	}
	return m.db.InTx(ctx, write)
}

func milestoneTemplate(row sqlc.MilestoneTemplate) milestones.Template {
	return milestones.Template{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Default:     row.IsDefault,
		Active:      row.Active,
	}
}

func milestoneTemplateItem(row sqlc.MilestoneTemplateItem) milestones.TemplateItem {
	return milestones.TemplateItem{
		ID:         row.ID,
		TemplateID: row.TemplateID,
		TypeID:     row.TypeID,
		Offset:     int(row.OffsetDays),
		Position:   int(row.Position),
	}
}
