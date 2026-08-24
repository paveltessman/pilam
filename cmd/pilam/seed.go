package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/postgres"
	"github.com/paveltessman/pilam/internal/seed"
)

const seedReport = `seed: users
  created:  %d
  skipped:  %d (an earlier run wrote them)
  password: %s

seed: catalog
  seasons: %d created, %d already there
  drops:   %d created, %d already there
  models:  %d created, %d already there
  photos:  %d written

seed: milestones
  types:     %d created, %d already there
  templates: %d created, %d already there
  steps:     %d created, %d already there
  calendars: %d created, %d already there
  facts:     %d stamped

Every seeded user logs in with that one password.
`

func runSeed(ctx context.Context, cfg config.Config, args []string) error {
	return seedAll(ctx, cfg, os.Stdout, args)
}

// seedAll loads the demo dataset and prints what it wrote.
func seedAll(ctx context.Context, cfg config.Config, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	users := fs.Int("users", 0, "how many employees to write; 0 writes the whole roster")
	models := fs.Int("models", 0, "how many models to write; 0 writes the whole season")
	domain := fs.String("domain", seed.DefaultDomain, "the mail domain the addresses are built under")
	passwd := fs.String("passwd", seed.DefaultPasswd, "the password every seeded user logs in with")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := postgres.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	mediaStore, err := media.New(cfg.Media)
	if err != nil {
		return err
	}

	clk := clock.New(cfg.Timezone)

	idGen := ids.NewDeterministic(seed.IDSeed)
	trail := audit.NewTrail(postgres.NewAudit(db), clk, idGen)

	// Nobody is logged in here. The seed writes one root first, and names that
	// root as the actor of every row after them.
	deps := seed.Deps{
		Auth:  auth.NewService(postgres.NewUsers(db), db, auth.NewThrottle(clk), idGen, trail),
		Clock: clk,
		Media: mediaStore,
		Catalog: catalog.NewService(catalog.Stores{
			Seasons: postgres.NewSeasons(db),
			Drops:   postgres.NewDrops(db),
			Models:  postgres.NewModels(db),
			Photos:  postgres.NewPhotos(db),
			Atomic:  db,
		}, idGen, trail),
		Milestones: milestones.NewService(milestones.Store{
			Types:      postgres.NewMilestoneTypes(db),
			Templates:  postgres.NewMilestoneTemplates(db),
			Milestones: postgres.NewMilestones(db),
			Atomic:     db,
		}, idGen, clk, trail),
	}

	report, err := seed.Run(ctx, deps, seed.Options{
		Users:  *users,
		Models: *models,
		Domain: *domain,
		Passwd: *passwd,
	})
	if err != nil {
		return err
	}

	rows, calendar := report.Catalog, report.Milestones
	_, err = fmt.Fprintf(out, seedReport,
		report.Users.Created, report.Users.Skipped, report.Users.Passwd,
		rows.Seasons.Created, rows.Seasons.Skipped,
		rows.Drops.Created, rows.Drops.Skipped,
		rows.Models.Created, rows.Models.Skipped,
		rows.Photos,
		calendar.Types.Created, calendar.Types.Skipped,
		calendar.Templates.Created, calendar.Templates.Skipped,
		calendar.Steps.Created, calendar.Steps.Skipped,
		calendar.Calendars.Created, calendar.Calendars.Skipped,
		calendar.Facts,
	)
	if err != nil {
		return fmt.Errorf("seed: the dataset is loaded, but printing the report failed: %w", err)
	}
	return nil
}
