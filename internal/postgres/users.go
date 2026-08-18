package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var (
	_ auth.Users  = (*Users)(nil)
	_ auth.Atomic = (*DB)(nil)
)

const (
	// SQLSTATE code for duplicate keys.
	uniqueViolation = "23505"

	// emailIndex is the unique index over lower(email), from 00002_users.sql.
	emailIndex = "app_user_email_key"
)

type Users struct {
	db *DB
}

func NewUsers(db *DB) *Users {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Users{db: db}
}

// ByID returns the user, or auth.ErrNoUser when there is no such row.
func (u *Users) ByID(ctx context.Context, id ids.ID) (auth.User, error) {
	row, err := u.db.queries(ctx).GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("%w: id %s", auth.ErrNoUser, id)
		}
		return auth.User{}, fmt.Errorf("postgres: loading user %s: %w", id, err)
	}
	return user(row), nil
}

// ByEmail matches the address case-insensitively.
func (u *Users) ByEmail(ctx context.Context, email string) (auth.User, error) {
	row, err := u.db.queries(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("%w: email %q", auth.ErrNoUser, email)
		}
		return auth.User{}, fmt.Errorf("postgres: loading user by email: %w", err)
	}
	return user(row), nil
}

// List returns every user, active and inactive, ordered by email.
func (u *Users) List(ctx context.Context) ([]auth.User, error) {
	rows, err := u.db.queries(ctx).ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing users: %w", err)
	}

	users := make([]auth.User, len(rows))
	for i, row := range rows {
		users[i] = user(row)
	}
	return users, nil
}

// Create writes one new row. A taken email returns auth.ErrEmailTaken.
func (u *Users) Create(ctx context.Context, in auth.User) error {
	params := sqlc.CreateUserParams{
		ID:            in.ID,
		Email:         in.Email,
		FirstName:     in.FirstName,
		LastName:      in.LastName,
		PasswdHash:    in.PasswdHash,
		Role:          string(in.Role),
		Active:        in.Active,
		SessionEpoch:  int32(in.SessionEpoch),
		PasswdExpired: in.PasswdExpired,
	}

	if err := u.db.queries(ctx).CreateUser(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating user %s: %w", in.ID, writeError(err))
	}
	return nil
}

// Update writes every field back to the row. Returns auth.ErrNoUser when the row
// is gone, and auth.ErrEmailTaken when another user already holds the new address.
func (u *Users) Update(ctx context.Context, in auth.User) error {
	params := sqlc.UpdateUserParams{
		ID:            in.ID,
		Email:         in.Email,
		FirstName:     in.FirstName,
		LastName:      in.LastName,
		PasswdHash:    in.PasswdHash,
		Role:          string(in.Role),
		Active:        in.Active,
		SessionEpoch:  int32(in.SessionEpoch),
		PasswdExpired: in.PasswdExpired,
	}

	written, err := u.db.queries(ctx).UpdateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating user %s: %w", in.ID, writeError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", auth.ErrNoUser, in.ID)
	}
	return nil
}

// user maps a row onto the domain type.
func user(row sqlc.AppUser) auth.User {
	return auth.User{
		ID:            row.ID,
		Email:         row.Email,
		FirstName:     row.FirstName,
		LastName:      row.LastName,
		PasswdHash:    row.PasswdHash,
		Role:          auth.Role(row.Role),
		Active:        row.Active,
		SessionEpoch:  int(row.SessionEpoch),
		PasswdExpired: row.PasswdExpired,
		UpdatedAt:     row.UpdatedAt,
	}
}

func writeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == emailIndex {
		return fmt.Errorf("%w: %w", auth.ErrEmailTaken, err)
	}
	return err
}
