package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres"
)

var userActions = map[string]func(context.Context, config.Config, io.Writer, []string) error{
	"add": userAdd,
}

func runUser(ctx context.Context, cfg config.Config, args []string) error {
	action := ""
	if len(args) > 0 {
		action = args[0]
	}
	run, ok := userActions[action]
	if !ok {
		return fmt.Errorf("user: unknown action %q, want add", action)
	}
	return run(ctx, cfg, os.Stdout, args[1:])
}

const userAddReport = `user created
  email:    %s
  name:     %s %s
  role:     %s
  password: %s

The user must change password at the first login.
`

// userAdd creates one user and prints the password it generated.
func userAdd(ctx context.Context, cfg config.Config, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	email := fs.String("email", "", "the address the user logs in with")
	name := fs.String("name", "", `the first and the last name, as "Ada Lovelace"`)
	root := fs.Bool("root", false, "give the user the root role, not the member one")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*email) == "" {
		return errors.New("user add: -email is required")
	}
	first, last, err := splitName(*name)
	if err != nil {
		return err
	}

	role := auth.MemberRole
	if *root {
		role = auth.RootRole
	}

	db, err := postgres.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	clk := clock.New(cfg.Timezone)
	idGen := ids.NewGenerator()

	// Nobody is logged in here, so the trail names the new user as the actor of
	// their own creation.
	service := auth.NewService(
		postgres.NewUsers(db),
		db,
		auth.NewThrottle(clk),
		idGen,
		audit.NewTrail(postgres.NewAudit(db), clk, idGen),
	)

	// The plain password lives in this variable and in the report below.
	user, passwd, err := service.Invite(ctx, auth.NewUser{
		Email:     *email,
		FirstName: first,
		LastName:  last,
		Role:      role,
	})
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			return fmt.Errorf("user add: another account already holds %s: %w", auth.NormalizeEmail(*email), err)
		}
		return fmt.Errorf("user add: %w", err)
	}

	_, err = fmt.Fprintf(out, userAddReport, user.Email, user.FirstName, user.LastName, user.Role, passwd)
	if err != nil {
		return fmt.Errorf("user add: the user exists, but printing the password failed: %w", err)
	}
	return nil
}

// splitName cuts one name into the two parts the row holds. The first word is
// the first name, and the rest is the last name.
func splitName(name string) (first, last string, err error) {
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return "", "", errors.New(`user add: -name needs a first and a last name, as -name "Ada Lovelace"`)
	}
	return parts[0], strings.Join(parts[1:], " "), nil
}
