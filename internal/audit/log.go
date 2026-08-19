package audit

import (
	"context"
	"fmt"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Reader reads the trail back.
type Reader interface {
	// ByEntity returns the entries of one entity, newest first, at most limit
	// of them. An entity with no entries yields an empty slice and no error.
	ByEntity(ctx context.Context, entity string, entityID ids.ID, limit int) ([]Entry, error)
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
	switch {
	case limit <= 0:
		limit = DefaultLimit
	case limit > MaxLimit:
		limit = MaxLimit
	}

	entries, err := l.reader.ByEntity(ctx, entity, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("audit: reading the trail of %s %s: %w", entity, entityID, err)
	}

	zone := clock.Zone(l.clock)
	for i := range entries {
		entries[i].At = entries[i].At.In(zone)
	}
	return entries, nil
}
