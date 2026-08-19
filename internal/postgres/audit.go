package postgres

import (
	"context"
	"fmt"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var (
	_ audit.Recorder = (*Audit)(nil)
	_ audit.Reader   = (*Audit)(nil)
)

// Audit stores the audit trail.
type Audit struct {
	db *DB
}

func NewAudit(db *DB) *Audit {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Audit{db: db}
}

// Record appends the entries to the trail.
//
// The insert joins the transaction the context carries, which is the one that
// holds the write the entries describe. A write which rolls back takes its
// entries with it. Where the context carries no transaction, Record opens one,
// so a set of entries lands whole or not at all.
func (a *Audit) Record(ctx context.Context, entries ...audit.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	write := func(ctx context.Context) error {
		queries := a.db.queries(ctx)
		for _, entry := range entries {
			if err := queries.CreateAuditEntry(ctx, params(entry)); err != nil {
				return fmt.Errorf("postgres: recording %s %s on %s %s: %w",
					entry.Entity, entry.Action, entry.Entity, entry.EntityID, err)
			}
		}
		return nil
	}
	return a.db.InTx(ctx, write)
}

// ByEntity returns the entries of one entity, newest first, at most limit of
// them. Index over (entity, entity_id, at DESC).
func (a *Audit) ByEntity(ctx context.Context, entity string, entityID ids.ID, limit int) ([]audit.Entry, error) {
	params := sqlc.ListAuditEntriesParams{
		Entity:   entity,
		EntityID: entityID,
		RowLimit: int32(limit),
	}

	rows, err := a.db.queries(ctx).ListAuditEntries(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing the trail of %s %s: %w", entity, entityID, err)
	}

	entries := make([]audit.Entry, len(rows))
	for i, row := range rows {
		entries[i] = entry(row)
	}
	return entries, nil
}

// entry maps a row onto the domain type. A NULL value column reads as the empty
// string.
func entry(row sqlc.AuditEntry) audit.Entry {
	return audit.Entry{
		ID:        row.ID,
		At:        row.At,
		ActorID:   row.ActorID,
		Entity:    row.Entity,
		EntityID:  row.EntityID,
		Action:    row.Action,
		FieldKey:  row.FieldKey,
		Old:       text(row.OldValue),
		New:       text(row.NewValue),
		RequestID: row.RequestID,
	}
}

func params(entry audit.Entry) sqlc.CreateAuditEntryParams {
	params := sqlc.CreateAuditEntryParams{
		ID:        entry.ID,
		At:        entry.At,
		ActorID:   entry.ActorID,
		Entity:    entry.Entity,
		EntityID:  entry.EntityID,
		Action:    entry.Action,
		FieldKey:  entry.FieldKey,
		OldValue:  value(entry.Old),
		NewValue:  value(entry.New),
		RequestID: entry.RequestID,
	}
	return params
}

// value maps a side of the change onto the column. A change with no value on
// that side stores NULL, so the column holds a value only where there is one.
func value(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// text is value the other way round: a NULL column reads as the empty string.
func text(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
