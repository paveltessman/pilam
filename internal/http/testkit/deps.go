// Package testkit builds the router a screen test serves requests against, and
// holds the fakes that stand in for the database.
//
// It imports internal/http, so a test that uses it must sit in an external test
// package: milestones_test rather than milestones. That is the rule Go states
// for a test whose helper imports a package which imports the package under
// test.
package testkit

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/session"
)

// Deps is the wiring NewRouter takes. The alias saves every test one import.
type Deps = pilamhttp.Deps

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

// NewDeps is what the composition root would have built, with the database
// swapped for a stub, media in a throwaway directory, and the log discarded.
func NewDeps(t *testing.T) Deps {
	t.Helper()
	d, _ := AuditedDeps(t)
	return d
}

// BaseDeps is everything that does not depend on the audit trail. AuditedDeps
// adds the service and the log, which share one trail.
func BaseDeps(t *testing.T) Deps {
	t.Helper()

	rows := newCatalogStore()
	d := Deps{
		DB:           stubPinger{},
		Logger:       logging.New(logging.Options{Format: logging.FormatText, Output: io.Discard}),
		IDs:          ids.NewGenerator(),
		Media:        MediaStore(t),
		SessionMgr:   session.New([]byte("test signing key"), time.Hour, clock.New(time.UTC)),
		CatalogSvc:   catalogServiceOn(t, &Trail{}, rows),
		MilestoneSvc: milestoneServiceOn(t, &Trail{}, rows),
	}
	return d
}

// AuditedDeps is NewDeps with a trail the test reads back. Every service
// records to it, and the screens read the same trail back through the log.
func AuditedDeps(t *testing.T) (Deps, *Trail) {
	t.Helper()

	trail := &Trail{}
	rows := newCatalogStore()
	d := BaseDeps(t)
	d.AuthSvc = AuthServiceOn(t, trail)
	d.CatalogSvc = catalogServiceOn(t, trail, rows)
	d.MilestoneSvc = milestoneServiceOn(t, trail, rows)
	d.AuditLog = audit.NewLog(trail, Clock())
	return d, trail
}

// MediaStore is a media store in a directory the test throws away.
func MediaStore(t *testing.T) media.Store {
	t.Helper()
	store, err := media.New(config.Media{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("building the media store: %v", err)
	}
	return store
}

// UnhealthyDeps is NewDeps with a database that answers the health check with
// err.
func UnhealthyDeps(t *testing.T, err error) Deps {
	t.Helper()
	d := NewDeps(t)
	d.DB = stubPinger{err: err}
	return d
}
