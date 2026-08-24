package audit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Reader reads the trail back.
type Reader interface {
	// ByEntities returns the entries of the rows named, newest first, at most
	// limit of them. A row with no entries adds nothing, and a set with no
	// entries at all yields an empty slice and no error.
	ByEntities(ctx context.Context, entity string, entityIDs []ids.ID, limit int) ([]Entry, error)
}

// Scope is one set of rows a trail read covers: an entity, and the identifiers
// of it.
//
// A card that carries the history of the rows under it reads several scopes at
// once. The model screen reads the model and its milestones together.
type Scope struct {
	Entity string
	IDs    []ids.ID
}

// Log is the read side of the trail: what happened to one entity, newest first.
// Trail writes the entries, and Log hands them to the screens.
type Log struct {
	reader Reader
	clock  clock.Clock
}

func NewLog(reader Reader, clk clock.Clock) *Log {
	switch {
	case reader == nil:
		panic("audit: nil reader")
	case clk == nil:
		panic("audit: nil clock")
	}
	return &Log{reader: reader, clock: clk}
}

// Entity returns the trail of one entity, newest first.
//
// A limit of zero or less reads DefaultLimit entries, and a limit above
// MaxLimit reads MaxLimit of them.
func (l *Log) Entity(ctx context.Context, entity string, entityID ids.ID, limit int) ([]Entry, error) {
	return l.Scopes(ctx, limit, Scope{Entity: entity, IDs: []ids.ID{entityID}})
}

// Scopes returns the trail of several sets of rows as one history, newest
// first, at most limit entries of the whole.
//
// A scope that names no row is read past. Scopes with no row at all yield an
// empty slice and no error.
func (l *Log) Scopes(ctx context.Context, limit int, scopes ...Scope) ([]Entry, error) {
	switch {
	case limit <= 0:
		limit = DefaultLimit
	case limit > MaxLimit:
		limit = MaxLimit
	}

	var entries []Entry
	for _, scope := range scopes {
		if len(scope.IDs) == 0 {
			continue
		}
		read, err := l.reader.ByEntities(ctx, scope.Entity, scope.IDs, limit)
		if err != nil {
			return nil, fmt.Errorf("audit: reading the trail of %d %s rows: %w",
				len(scope.IDs), scope.Entity, err)
		}
		entries = append(entries, read...)
	}

	// Every scope was read newest first, and the merge of two of them is not.
	if len(scopes) > 1 {
		slices.SortFunc(entries, newestFirst)
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	zone := clock.Zone(l.clock)
	for i := range entries {
		entries[i].At = entries[i].At.In(zone)
	}
	return entries, nil
}

// newestFirst orders two entries the way a card reads them. Two entries stamped
// alike break the tie by identifier, which is the order the store itself uses.
func newestFirst(a, b Entry) int {
	if !a.At.Equal(b.At) {
		return b.At.Compare(a.At)
	}
	return strings.Compare(b.ID.String(), a.ID.String())
}
