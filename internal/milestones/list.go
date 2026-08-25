package milestones

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// This code owns the milestone list: the row it shows across a whole season,
// and the rule that picks and orders the rows.

// ListRow is one milestone of the list: the row, the state it holds today, and
// the names the screen reads beside it.
//
// The names ride along with the read, because the list crosses every model of a
// season and naming them one by one is a query per row.
type ListRow struct {
	Milestone
	State State

	// TypeName is the name of the step, Article names the model, and the
	// two drop fields name the drop that holds it.
	TypeName string
	Article  string
	DropID   ids.ID
	DropName string
}

// ListParams is what the list reads.
type ListParams struct {
	SeasonID ids.ID
	DropID   ids.ID
	TypeID   ids.ID

	// States keeps the rows that hold one of the states named. No state at all
	// keeps every row.
	States []State

	// Search is free text on the article of the model.
	Search string
}

func LateAndDue() []State { return []State{StateLate, StateDue} }

// NoCalendar is how many models in the drop have no calendar.
type NoCalendar struct {
	DropID   ids.ID
	DropName string
	Models   int
}

// ListMilestones is the milestone list: the steps of one season the filter
// keeps, each with the state it holds today, worst slip first.
func (s *Service) ListMilestones(ctx context.Context, filter ListParams) ([]ListRow, error) {
	held, err := s.milestones.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing the milestones of season %s: %w", filter.SeasonID, err)
	}
	return Listed(held, filter, s.clock.Today()), nil
}

// ModelsWithoutCalendar counts the models of one season that hold no calendar,
// per drop. The zero drop identifier counts every drop of the season.
func (s *Service) ModelsWithoutCalendar(ctx context.Context, seasonID, dropID ids.ID) ([]NoCalendar, error) {
	counts, err := s.milestones.WithoutCalendar(ctx, seasonID, dropID)
	if err != nil {
		return nil, fmt.Errorf("milestone: counting the models of season %s with no calendar: %w", seasonID, err)
	}
	return counts, nil
}

// Listed is the rule of the list: the rows the filter keeps, each with the
// state it holds today, worst slip first.
//
// held is what the store answered for the season, the drop and the type. The
// state and the search run here, because the state is a rule of this package
// and the search is free text over a list the store already narrowed.
//
// Two rows of the same slip break the tie by plan date, then by article, then
// by identifier, so two reads of one season never disagree.
func Listed(held []ListRow, filter ListParams, today time.Time) []ListRow {
	terms := strings.Fields(strings.ToLower(filter.Search))

	rows := make([]ListRow, 0, len(held))
	for _, row := range held {
		row.State = row.Milestone.State(today)
		if !holdsState(filter.States, row.State) || !matches(row.Article, terms) {
			continue
		}
		rows = append(rows, row)
	}

	slices.SortFunc(rows, worstSlipFirst)
	return rows
}

// worstSlipFirst orders two rows of the list.
func worstSlipFirst(a, b ListRow) int {
	if slip := b.Slip() - a.Slip(); slip != 0 {
		return slip
	}
	if !date.Equal(a.Plan, b.Plan) {
		return a.Plan.Compare(b.Plan)
	}
	if byArticle := strings.Compare(a.Article, b.Article); byArticle != 0 {
		return byArticle
	}
	return strings.Compare(a.ID.String(), b.ID.String())
}

// holdsState reports whether the filter names the state. A filter that names no
// state at all keeps every row.
func holdsState(states []State, state State) bool {
	return len(states) == 0 || slices.Contains(states, state)
}

// matches reports whether the article answers every term of the search. A term
// answers when it is part of the article, so "art 1" finds "ART-1" and "art 9"
// finds nothing.
func matches(article string, terms []string) bool {
	article = strings.ToLower(article)
	for _, term := range terms {
		if !strings.Contains(article, term) {
			return false
		}
	}
	return true
}
