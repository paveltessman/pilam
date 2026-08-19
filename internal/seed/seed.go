// Package seed loads the demo dataset.
//
// Every write goes through the service the application itself writes through,
// so a seeded row carries the audit entries a real row carries.
package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/password"
)

const (
	DefaultDomain = "pilam.example"
	DefaultPasswd = "demo-password"
)

const IDSeed uint64 = 2027

// Options is what the caller varies between runs.
type Options struct {
	// Users is how many employees to write. Zero writes the whole roster.
	Users int

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
	Users UsersReport
}

// Run loads the whole demo dataset. It is safe to run twice: a row that is
// already there is left as it stands, and counted as skipped.
func Run(ctx context.Context, authSvc *auth.Service, opts Options) (Report, error) {
	if authSvc == nil {
		return Report{}, errors.New("seed: nil auth service")
	}

	opts = opts.withDefaults()
	if err := opts.check(); err != nil {
		return Report{}, err
	}

	users, err := seedUsers(ctx, authSvc, opts)
	if err != nil {
		return Report{}, err
	}

	return Report{Users: users}, nil
}
