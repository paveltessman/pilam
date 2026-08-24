package milestones

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Template is an ordered list of milestone types with their offsets. A calendar
// is built from one.
type Template struct {
	ID          ids.ID
	Name        string
	Description string

	// Default marks the template a new model starts with. At most one template
	// carries it.
	Default bool
	Active  bool
}

// TemplateItem is one step of a template: a milestone type, the days it runs
// from the target date of the drop, and where it sits on the screen.
type TemplateItem struct {
	ID         ids.ID
	TemplateID ids.ID
	TypeID     ids.ID

	// Offset is days from the target date of the drop. Zero or negative.
	Offset   int
	Position int
}

// TemplatesStore is the store of templates and of the items they hold. The item
// is part of the template, so one port carries both.
//
// Writes return ErrNameTaken on a duplicate name.
type TemplatesStore interface {
	// ByID returns ErrNoTemplate when nothing matches.
	ByID(ctx context.Context, id ids.ID) (Template, error)

	// List returns every template, active and inactive, ordered by name.
	List(ctx context.Context) ([]Template, error)

	Create(ctx context.Context, in Template) error
	Update(ctx context.Context, in Template) error

	// ClearDefault drops the default flag of every template but the one named.
	// The write that names a new default runs it first, because the store holds
	// one default at a time.
	ClearDefault(ctx context.Context, keep ids.ID) error

	// Items returns the items of one template, in position order.
	Items(ctx context.Context, templateID ids.ID) ([]TemplateItem, error)

	// AddItem returns ErrTypeInTemplate when the template already holds the type.
	AddItem(ctx context.Context, in TemplateItem) error
	UpdateItem(ctx context.Context, in TemplateItem) error

	// RemoveItem returns ErrNoItem when the row is gone.
	RemoveItem(ctx context.Context, itemID ids.ID) error

	// ReorderItems writes position i to the i-th identifier of ordered. The
	// caller states a whole template.
	ReorderItems(ctx context.Context, ordered []ids.ID) error
}

type TemplateCreateParams struct {
	Name        string
	Description string
	Default     bool
}

type TemplateUpdateParams struct {
	Name        string
	Description string
	Default     bool
	Active      bool
}

// Template returns one milestone template.
func (s *Service) Template(ctx context.Context, templateID ids.ID) (Template, error) {
	template, err := s.templates.ByID(ctx, templateID)
	if err != nil {
		return Template{}, fmt.Errorf("milestone: loading template %s: %w", templateID, err)
	}
	return template, nil
}

// ListTemplates returns every template, active and inactive, by name.
func (s *Service) ListTemplates(ctx context.Context) ([]Template, error) {
	templates, err := s.templates.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing templates: %w", err)
	}
	return templates, nil
}

// TemplateItems returns the items of one template, in the order the screen
// shows them.
func (s *Service) TemplateItems(ctx context.Context, templateID ids.ID) ([]TemplateItem, error) {
	items, err := s.templates.Items(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
	}
	return items, nil
}

// CreateTemplate writes a new template and returns the row it wrote. The
// template starts empty: the editor adds the items.
func (s *Service) CreateTemplate(ctx context.Context, in TemplateCreateParams) (Template, error) {
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)

	if err := checkNaming(name, description); err != nil {
		return Template{}, err
	}

	template := Template{
		ID:          s.ids.New(),
		Name:        name,
		Description: description,
		Default:     in.Default,
		Active:      true,
	}

	write := func(ctx context.Context) error {
		if template.Default {
			if err := s.templates.ClearDefault(ctx, template.ID); err != nil {
				return err
			}
		}
		if err := s.templates.Create(ctx, template); err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: template.ID,
			Action:   audit.ActionCreated,
			New:      template.Name,
		})
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Template{}, err
	}
	return template, nil
}

// UpdateTemplate writes the edited fields and records one trail entry per field
// that moved. It writes nothing when nothing moved.
func (s *Service) UpdateTemplate(ctx context.Context, templateID ids.ID, in TemplateUpdateParams) error {
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)

	var v validate.Validator
	v.Merge(checkNaming(name, description))
	// An inactive template leaves the pickers, so it cannot be the one a new
	// model starts with.
	v.Check(!in.Default || in.Active, FieldDefault, validate.NotAllowed)
	if err := v.Err(); err != nil {
		return err
	}

	template, err := s.templates.ByID(ctx, templateID)
	if err != nil {
		return fmt.Errorf("milestone: loading template %s: %w", templateID, err)
	}

	var changes []audit.Change
	record := func(action, field, old, next string) {
		changes = append(changes, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: template.ID,
			Action:   action,
			FieldKey: field,
			Old:      old,
			New:      next,
		})
	}

	if name != template.Name {
		record(audit.ActionChanged, FieldName, template.Name, name)
		template.Name = name
	}
	if description != template.Description {
		record(audit.ActionChanged, FieldDescription, template.Description, description)
		template.Description = description
	}
	if in.Default != template.Default {
		record(audit.ActionChanged, FieldDefault,
			strconv.FormatBool(template.Default), strconv.FormatBool(in.Default))
		template.Default = in.Default
	}
	if in.Active != template.Active {
		record(audit.Activation(in.Active), FieldActive,
			strconv.FormatBool(template.Active), strconv.FormatBool(in.Active))
		template.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	write := func(ctx context.Context) error {
		if template.Default {
			if err := s.templates.ClearDefault(ctx, template.ID); err != nil {
				return err
			}
		}
		if err := s.templates.Update(ctx, template); err != nil {
			return err
		}
		return s.RecordTrail(ctx, changes...)
	}
	return s.atomic.InTx(ctx, write)
}

// AddTemplateItem appends one step to a template. The step lands at the end of
// the list.
//
// It refuses a type the template already holds, and a type that is not active.
func (s *Service) AddTemplateItem(ctx context.Context, templateID, typeID ids.ID, offset int) (TemplateItem, error) {
	if err := checkOffset(FieldOffset, offset); err != nil {
		return TemplateItem{}, err
	}

	if _, err := s.templates.ByID(ctx, templateID); err != nil {
		return TemplateItem{}, fmt.Errorf("milestone: loading template %s: %w", templateID, err)
	}

	// A type that is not there and a type that is not active are the same answer:
	// the picker does not offer either one.
	milestoneType, err := s.types.ByID(ctx, typeID)
	if errors.Is(err, ErrNoType) {
		return TemplateItem{}, validate.Fail(FieldType, validate.NotAllowed)
	}
	if err != nil {
		return TemplateItem{}, fmt.Errorf("milestone: loading type %s: %w", typeID, err)
	}
	if !milestoneType.Active {
		return TemplateItem{}, validate.Fail(FieldType, validate.NotAllowed)
	}

	var item TemplateItem
	write := func(ctx context.Context) error {
		held, err := s.templates.Items(ctx, templateID)
		if err != nil {
			return fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
		}
		if slices.ContainsFunc(held, func(i TemplateItem) bool { return i.TypeID == typeID }) {
			return fmt.Errorf("%w: type %s in template %s", ErrTypeInTemplate, typeID, templateID)
		}

		item = TemplateItem{
			ID:         s.ids.New(),
			TemplateID: templateID,
			TypeID:     typeID,
			Offset:     offset,
			Position:   len(held),
		}
		if err := s.templates.AddItem(ctx, item); err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: templateID,
			Action:   audit.ActionItemAdded,
			FieldKey: FieldOffset,
			New:      itemValue(milestoneType.Name, offset),
		})
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return TemplateItem{}, err
	}
	return item, nil
}

// SetTemplateItemOffset moves one item of a template. The offset is the days
// from the target date of the drop.
func (s *Service) SetTemplateItemOffset(ctx context.Context, templateID, itemID ids.ID, offset int) error {
	from := func([]TemplateItem, int) (int, error) { return offset, nil }
	return s.moveItem(ctx, templateID, itemID, FieldOffset, from)
}

// SetTemplateItemGap moves one item of a template by the gap: the days from the
// item above it. The app stores the offset the gap works out to.
//
// The first item of a template has no item above it, so it carries no gap.
func (s *Service) SetTemplateItemGap(ctx context.Context, templateID, itemID ids.ID, gap int) error {
	from := func(items []TemplateItem, at int) (int, error) {
		if at == 0 {
			return 0, validate.Fail(FieldGap, validate.NotAllowed)
		}
		return items[at-1].Offset + gap, nil
	}
	return s.moveItem(ctx, templateID, itemID, FieldGap, from)
}

// RemoveTemplateItem deletes one step of a template and closes the gap it
// leaves in the order.
func (s *Service) RemoveTemplateItem(ctx context.Context, templateID, itemID ids.ID) error {
	write := func(ctx context.Context) error {
		held, err := s.templates.Items(ctx, templateID)
		if err != nil {
			return fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
		}

		at := slices.IndexFunc(held, func(i TemplateItem) bool { return i.ID == itemID })
		if at < 0 {
			return fmt.Errorf("%w: id %s in template %s", ErrNoItem, itemID, templateID)
		}
		removed := held[at]

		if err := s.templates.RemoveItem(ctx, itemID); err != nil {
			return err
		}
		left := slices.Delete(slices.Clone(held), at, at+1)
		if err := s.templates.ReorderItems(ctx, itemIDs(left)); err != nil {
			return err
		}

		name, err := s.typeName(ctx, removed.TypeID)
		if err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: templateID,
			Action:   audit.ActionItemRemoved,
			FieldKey: FieldOffset,
			Old:      itemValue(name, removed.Offset),
		})
	}
	return s.atomic.InTx(ctx, write)
}

// ReorderTemplateItems writes the items of a template in the stated order.
//
// ordered must name every item of the template exactly once, and nothing else.
// Anything else returns ErrItemOrder and writes nothing.
func (s *Service) ReorderTemplateItems(ctx context.Context, templateID ids.ID, ordered []ids.ID) error {
	write := func(ctx context.Context) error {
		held, err := s.templates.Items(ctx, templateID)
		if err != nil {
			return fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
		}

		was := itemIDs(held)
		if !rearranges(was, ordered) {
			return fmt.Errorf("%w: template %s holds %d items, the order names %d",
				ErrItemOrder, templateID, len(was), len(ordered))
		}
		if slices.Equal(was, ordered) {
			return nil
		}

		if err := s.templates.ReorderItems(ctx, ordered); err != nil {
			return err
		}

		names, err := s.typeNames(ctx)
		if err != nil {
			return err
		}
		byID := make(map[ids.ID]TemplateItem, len(held))
		for _, item := range held {
			byID[item.ID] = item
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: templateID,
			Action:   audit.ActionItemsReordered,
			FieldKey: FieldItemOrder,
			Old:      listValue(was, byID, names),
			New:      listValue(ordered, byID, names),
		})
	}
	return s.atomic.InTx(ctx, write)
}

// moveItem is what the offset edit and the gap edit share. offsetOf states the
// offset the item lands on, reading the items the template holds now.
func (s *Service) moveItem(
	ctx context.Context,
	templateID, itemID ids.ID,
	field string,
	offsetOf func(items []TemplateItem, at int) (int, error),
) error {
	write := func(ctx context.Context) error {
		held, err := s.templates.Items(ctx, templateID)
		if err != nil {
			return fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
		}

		at := slices.IndexFunc(held, func(i TemplateItem) bool { return i.ID == itemID })
		if at < 0 {
			return fmt.Errorf("%w: id %s in template %s", ErrNoItem, itemID, templateID)
		}

		offset, err := offsetOf(held, at)
		if err != nil {
			return err
		}
		if err := checkOffset(field, offset); err != nil {
			return err
		}

		item := held[at]
		if item.Offset == offset {
			return nil
		}
		was := item.Offset
		item.Offset = offset
		if err := s.templates.UpdateItem(ctx, item); err != nil {
			return err
		}

		name, err := s.typeName(ctx, item.TypeID)
		if err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestoneTemplate,
			EntityID: templateID,
			Action:   audit.ActionChanged,
			FieldKey: FieldOffset,
			Old:      itemValue(name, was),
			New:      itemValue(name, offset),
		})
	}
	return s.atomic.InTx(ctx, write)
}

// typeName reads the short name one item carries, for the trail.
func (s *Service) typeName(ctx context.Context, typeID ids.ID) (string, error) {
	milestoneType, err := s.types.ByID(ctx, typeID)
	if err != nil {
		return "", fmt.Errorf("milestone: loading type %s: %w", typeID, err)
	}
	return milestoneType.Name, nil
}

// typeNames is the short name of every type, keyed by the type.
func (s *Service) typeNames(ctx context.Context) (map[ids.ID]string, error) {
	types, err := s.types.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing types: %w", err)
	}
	names := make(map[ids.ID]string, len(types))
	for _, milestoneType := range types {
		names[milestoneType.ID] = milestoneType.Name
	}
	return names, nil
}

// checkOffset states the bound an offset lies within. field names the input the
// rejection lands on: the offset box, or the gap box the offset came from.
func checkOffset(field string, offset int) error {
	var v validate.Validator
	if v.AtMost(field, offset, MaxOffset) {
		v.AtLeast(field, offset, MinOffset)
	}
	return v.Err()
}

// itemValue renders one item for the trail: the step, and the offset it holds.
func itemValue(name string, offset int) string {
	return name + " (" + strconv.Itoa(offset) + ")"
}

// listValue renders an order for the trail: the steps it names, in order.
func listValue(order []ids.ID, byID map[ids.ID]TemplateItem, names map[ids.ID]string) string {
	parts := make([]string, len(order))
	for i, id := range order {
		parts[i] = names[byID[id].TypeID]
	}
	return strings.Join(parts, ", ")
}

// rearranges reports whether ordered holds the same identifiers as was, each of
// them once.
func rearranges(was, ordered []ids.ID) bool {
	if len(was) != len(ordered) {
		return false
	}
	seen := make(map[ids.ID]bool, len(ordered))
	for _, id := range ordered {
		if seen[id] || !slices.Contains(was, id) {
			return false
		}
		seen[id] = true
	}
	return true
}

func itemIDs(items []TemplateItem) []ids.ID {
	out := make([]ids.ID, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}
