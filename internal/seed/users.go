package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// employee is one line of the roster.
//
// local is the part of the address before the "@", written out rather than
// transliterated from the name, so that the address of a given employee never
// moves.
type employee struct {
	first  string
	last   string
	local  string
	role   auth.Role
	active bool
}

func (e employee) email(domain string) string { return e.local + "@" + domain }

// rootAt is where the roster holds the root the whole dataset is created by.
// The seed writes that root first, so every user after them has an actor the
// trail can name.
const rootAt = 0

// roster is the demo company.
var roster = []employee{
	// The two accounts that reach the user screens. The first of them is the
	// actor of the whole dataset, and rootAt points at them.
	{"Марина", "Ковалёва", "marina.kovaleva", auth.RootRole, true},
	{"Артём", "Соколов", "artem.sokolov", auth.RootRole, true},

	// Product managers, the owners of a model.
	{"Ольга", "Тарасова", "olga.tarasova", auth.MemberRole, true},
	{"Дмитрий", "Лебедев", "dmitry.lebedev", auth.MemberRole, true},
	{"Екатерина", "Волкова", "ekaterina.volkova", auth.MemberRole, true},
	{"Роман", "Медведев", "roman.medvedev", auth.MemberRole, true},

	// Designers.
	{"Анна", "Морозова", "anna.morozova", auth.MemberRole, true},
	{"Юлия", "Зайцева", "yulia.zaytseva", auth.MemberRole, true},
	{"Дарья", "Крылова", "daria.krylova", auth.MemberRole, true},
	{"Виктория", "Сорокина", "victoria.sorokina", auth.MemberRole, true},
	{"Полина", "Жукова", "polina.zhukova", auth.MemberRole, true},

	// Pattern makers.
	{"Ирина", "Панова", "irina.panova", auth.MemberRole, true},
	{"Светлана", "Белова", "svetlana.belova", auth.MemberRole, true},
	{"Наталья", "Фомина", "natalia.fomina", auth.MemberRole, true},
	{"Ксения", "Титова", "ksenia.titova", auth.MemberRole, true},

	// Garment technologists.
	{"Сергей", "Новиков", "sergey.novikov", auth.MemberRole, true},
	{"Максим", "Гусев", "maxim.gusev", auth.MemberRole, true},
	{"Алексей", "Королёв", "alexey.korolev", auth.MemberRole, true},
	{"Тимур", "Ахметов", "timur.akhmetov", auth.MemberRole, true},

	// Sourcing and production planning.
	{"Павел", "Егоров", "pavel.egorov", auth.MemberRole, true},
	{"Егор", "Кузнецов", "egor.kuznetsov", auth.MemberRole, true},
	{"Алина", "Шестакова", "alina.shestakova", auth.MemberRole, true},

	// Two people who left. Their rows stay, and their access does not, which is
	// what the users screen shows a deactivated account for.
	{"Людмила", "Осипова", "lyudmila.osipova", auth.MemberRole, false},
	{"Никита", "Громов", "nikita.gromov", auth.MemberRole, false},
}

// UsersReport is what the users part of one run wrote.
type UsersReport struct {
	// Created counts the employees this run wrote.
	Created int

	// Skipped counts the employees another run already wrote.
	Skipped int

	// Passwd is the password every seeded employee logs in with.
	Passwd string
}

// seedUsers writes the employees, and reports what it wrote. It returns the
// context the rest of the dataset is written under.
//
// The root at rootAt goes in first and creates themselves, because nobody is
// logged in when the seed runs. From there on the context names that root, so
// the trail reads the way it reads in a real company: one administrator opened
// every account.
func seedUsers(ctx context.Context, authSvc *auth.Service, opts Options) (UsersReport, context.Context, error) {
	people := roster
	if opts.Users > 0 {
		people = people[:opts.Users]
	}

	report := UsersReport{Passwd: opts.Passwd}
	for i, person := range people {
		email := person.email(opts.Domain)

		user, err := authSvc.Create(ctx, auth.NewUser{
			Email:     email,
			FirstName: person.first,
			LastName:  person.last,
			Role:      person.role,
			Passwd:    opts.Passwd,
		})
		switch {
		case errors.Is(err, auth.ErrEmailTaken), errors.Is(err, auth.ErrIDTaken):
			report.Skipped++

		case err != nil:
			return report, ctx, fmt.Errorf("seed: creating user %s: %w", email, err)

		default:
			// A user is born active, so the two who left are deactivated after
			// the fact. The trail then reads the way it reads for a real leaver.
			if !person.active {
				err := authSvc.Update(ctx, user.ID, auth.UpdateParams{
					FirstName: user.FirstName,
					LastName:  user.LastName,
					Role:      user.Role,
					Active:    false,
				})
				if err != nil {
					return report, ctx, fmt.Errorf("seed: deactivating user %s: %w", email, err)
				}
			}

			report.Created++
		}

		if i == rootAt {
			ctx, err = runAsRoot(ctx, authSvc, user, email)
			if err != nil {
				return report, ctx, err
			}
		}
	}

	return report, ctx, nil
}

// runAsRoot returns a context that names the demo root as the actor of every
// write made under it.
//
// written is the row this run wrote. It is the zero user where an earlier run
// already held the address, and the search then finds the root that run wrote.
// Both runs name one actor this way.
//
// A store that holds neither leaves the context as it stands. Each user then
// creates themselves, which is what the trail held before the root was there.
func runAsRoot(ctx context.Context, authSvc *auth.Service, written auth.User, email string) (context.Context, error) {
	if written.ID != ids.Nil {
		return auth.NewContext(ctx, written.Identity()), nil
	}

	accounts, err := authSvc.List(ctx, email)
	if err != nil {
		return ctx, fmt.Errorf("seed: looking up the root %s: %w", email, err)
	}

	for _, account := range accounts {
		if account.Email != email {
			continue
		}
		identity := auth.Identity{
			UserID:    account.ID,
			Email:     account.Email,
			FirstName: account.FirstName,
			LastName:  account.LastName,
			Role:      account.Role,
		}
		return auth.NewContext(ctx, identity), nil
	}
	return ctx, nil
}
