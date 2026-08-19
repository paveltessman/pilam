package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres"
	"github.com/paveltessman/pilam/internal/seed"
)

const seedReport = `seed: users
  created:  %d
  skipped:  %d (an earlier run wrote them)
  password: %s

Every seeded user logs in with that one password.
`

func runSeed(ctx context.Context, cfg config.Config, args []string) error {
	return seedAll(ctx, cfg, os.Stdout, args)
}

// seedAll loads the demo dataset and prints what it wrote.
func seedAll(ctx context.Context, cfg config.Config, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	users := fs.Int("users", 0, "how many employees to write; 0 writes the whole roster")
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

	clk := clock.New(cfg.Timezone)

	idGen := ids.NewDeterministic(seed.IDSeed)

	// Nobody is logged in here. The seed writes one root first, and names that
	// root as the actor of every user after them.
	authSvc := auth.NewService(
		postgres.NewUsers(db),
		db,
		auth.NewThrottle(clk),
		idGen,
		audit.NewTrail(postgres.NewAudit(db), clk, idGen),
	)

	report, err := seed.Run(ctx, authSvc, seed.Options{
		Users:  *users,
		Domain: *domain,
		Passwd: *passwd,
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, seedReport, report.Users.Created, report.Users.Skipped, report.Users.Passwd)
	if err != nil {
		return fmt.Errorf("seed: the dataset is loaded, but printing the report failed: %w", err)
	}
	return nil
}
