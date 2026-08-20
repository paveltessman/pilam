// Package seed loads the demo dataset.
//
// Every write goes through the service the application itself writes through,
// so a seeded row carries the audit entries a real row carries.
//
// The seed is deterministic. The identifiers come from an injected generator,
// the dates are all relative to the clock, the placeholder photos are painted
// from the row index alone, and nothing draws on a random source. Two runs from
// an empty database write the same rows.
package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/password"
)

const (
	DefaultDomain = "pilam.example"
	DefaultPasswd = "demo-password"
)

const IDSeed uint64 = 2027

// Deps is what the seed writes through.
type Deps struct {
	Auth    *auth.Service
	Catalog *catalog.Service
	Media   media.Store
	Clock   clock.Clock
}

// check refuses a dependency the seed can't work without.
func (d Deps) check() error {
	switch {
	case d.Auth == nil:
		return errors.New("seed: nil auth service")
	case d.Catalog == nil:
		return errors.New("seed: nil catalog service")
	case d.Media == nil:
		return errors.New("seed: nil media store")
	case d.Clock == nil:
		return errors.New("seed: nil clock")
	}
	return nil
}

// Options is what the caller varies between runs.
type Options struct {
	// Users is how many employees to write. Zero writes the whole roster.
	Users int

	// Models is how many models to write, counted over the season in drop
	// order. Zero writes them all.
	//
	// The season and all three drops are written whatever this holds, because a
	// drop with no model is still part of the spine.
	Models int

	// Domain is the mail domain the addresses are built under. Empty means
	// DefaultDomain.
	Domain string

	// Passwd is the password every seeded employee logs in with. Empty means
	// DefaultPasswd.
	Passwd string
}

// withDefaults fills in what the caller left empty.
func (o Options) withDefaults() Options {
	if o.Domain == "" {
		o.Domain = DefaultDomain
	}
	if o.Passwd == "" {
		o.Passwd = DefaultPasswd
	}
	return o
}

// check refuses options the seed can't work from.
func (o Options) check() error {
	switch {
	case o.Users < 0:
		return fmt.Errorf("seed: %d users is not a count", o.Users)
	case o.Users > len(roster):
		return fmt.Errorf("seed: the roster holds %d employees, %d were asked for", len(roster), o.Users)
	case o.Models < 0:
		return fmt.Errorf("seed: %d models is not a count", o.Models)
	case o.Models > plannedModels():
		return fmt.Errorf("seed: the season holds %d models, %d were asked for", plannedModels(), o.Models)
	}
	if err := password.Check(auth.FieldPasswd, o.Passwd); err != nil {
		return fmt.Errorf("seed: the password does not pass the policy: %w", err)
	}
	if o.Domain == "" {
		return errors.New("seed: the mail domain is empty")
	}
	return nil
}

// Report is what one run wrote.
type Report struct {
	Users   UsersReport
	Catalog CatalogReport
}

// Run loads the whole demo dataset. It is safe to run twice: a row that is
// already there is left as it stands, and counted as skipped.
//
// The users go in first. They give the catalog its actor.
func Run(ctx context.Context, deps Deps, opts Options) (Report, error) {
	if err := deps.check(); err != nil {
		return Report{}, err
	}

	opts = opts.withDefaults()
	if err := opts.check(); err != nil {
		return Report{}, err
	}

	users, ctx, err := seedUsers(ctx, deps.Auth, opts)
	if err != nil {
		return Report{}, err
	}

	rows, err := seedCatalog(ctx, deps, opts)
	if err != nil {
		return Report{}, err
	}

	return Report{Users: users, Catalog: rows}, nil
}
