// Package auth owns who the user is.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/password"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Role is what an identity is allowed to do.
type Role string

const (
	MemberRole Role = "member"
	RootRole   Role = "root"
)

func (r Role) Valid() bool { return r == MemberRole || r == RootRole }

// Allows reports whether the role covers what required asks for.
func (r Role) Allows(required Role) bool { return r == RootRole || r == required }

// The fields a rejection from this package names. The view layer maps them onto
// its own inputs.
const (
	FieldEmail       = "email"
	FieldFirstName   = "first_name"
	FieldLastName    = "last_name"
	FieldRole        = "role"
	FieldActive      = "active"
	FieldPasswd      = "passwd"
	FieldCurrentPass = "current_passwd"
	FieldNewPass     = "new_passwd"
)

var (
	ErrNotResolved = errors.New("auth: user not resolved")
	ErrStaleEpoch  = errors.New("auth: stale session epoch")
	ErrInactive    = errors.New("auth: user is deactivated")
	ErrNoUser      = errors.New("auth: no such user")
	ErrEmailTaken  = errors.New("auth: email already taken")

	// ErrSelfLockout is a user removing their own access: deactivating
	// themselves, or dropping their own root role. Somebody else must do it,
	// so that the last root can't lock the section against everybody.
	ErrSelfLockout = errors.New("auth: a user can't remove their own access")
)

// Identity is the authenticated user, as every layer below transport sees them.
type Identity struct {
	UserID        ids.ID
	Email         string
	FirstName     string
	LastName      string
	Role          Role
	PasswdExpired bool
}

// IsZero reports whether the request is anonymous (not logged in).
func (i Identity) IsZero() bool { return i.UserID == ids.Nil }

func (i Identity) LogValue() slog.Value {
	if i.IsZero() {
		return slog.StringValue("anonymous")
	}
	return slog.StringValue(i.Email)
}

type User struct {
	ID            ids.ID
	Email         string
	FirstName     string
	LastName      string
	PasswdHash    string
	Role          Role
	Active        bool
	SessionEpoch  int
	PasswdExpired bool
	UpdatedAt     time.Time
}

func (u User) Identity() Identity {
	identity := Identity{
		UserID:        u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Role:          u.Role,
		PasswdExpired: u.PasswdExpired,
	}
	return identity
}

func (u User) Principal() Principal {
	return Principal{UserID: u.ID, Epoch: u.SessionEpoch}
}

// Account is one user as the users section shows them.
type Account struct {
	ID        ids.ID
	Email     string
	FirstName string
	LastName  string
	Role      Role
	Active    bool
	UpdatedAt time.Time
}

func (u User) Account() Account {
	account := Account{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
		Active:    u.Active,
		UpdatedAt: u.UpdatedAt,
	}
	return account
}

// Users is the store of user rows. ByID and ByEmail return ErrNoUser when
// nothing matches, and Create returns ErrEmailTaken on a duplicate address.
type Users interface {
	ByID(ctx context.Context, id ids.ID) (User, error)
	ByEmail(ctx context.Context, email string) (User, error)

	// List returns every user, active and inactive, ordered by email.
	List(ctx context.Context) ([]User, error)

	Create(ctx context.Context, user User) error
	Update(ctx context.Context, user User) error
}

// Atomic runs a unit of work in one transaction. The transaction travels in the
// context, so a port call inside fn joins it without carrying a handle.
type Atomic interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Throttle holds back an account that keeps failing to log in.
// The key is the normalized submitted login.
type Throttle interface {
	// Allow reports whether key may try a password now.
	Allow(key string) bool

	// Fail records one wrong attempt.
	Fail(key string)

	// Reset clears the record after a login that held.
	Reset(key string)
}

type Service struct {
	users    Users
	atomic   Atomic
	throttle Throttle
	ids      ids.Generator
	trail    *audit.Trail
}

func NewService(users Users, atomic Atomic, throttle Throttle, gen ids.Generator, trail *audit.Trail) *Service {
	switch {
	case users == nil:
		panic("auth: nil users store")
	case atomic == nil:
		panic("auth: nil transaction runner")
	case throttle == nil:
		panic("auth: nil throttle")
	case gen == nil:
		panic("auth: nil id generator")
	case trail == nil:
		panic("auth: nil audit trail")
	}
	return &Service{users: users, atomic: atomic, throttle: throttle, ids: gen, trail: trail}
}

// Resolve returns the identity a session subject names.
func (s *Service) Resolve(ctx context.Context, subject string) (Identity, error) {
	principal, err := ParsePrincipal(subject)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrNotResolved, err)
	}

	user, err := s.users.ByID(ctx, principal.UserID)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: loading user %s: %w", ErrNotResolved, principal.UserID, err)
	}

	if !user.Active {
		return Identity{}, fmt.Errorf("%w: %w: %s", ErrNotResolved, ErrInactive, user.ID)
	}

	if user.SessionEpoch != principal.Epoch {
		return Identity{}, fmt.Errorf("%w: %w: subject carries %d, the row holds %d",
			ErrNotResolved, ErrStaleEpoch, principal.Epoch, user.SessionEpoch)
	}

	return user.Identity(), nil
}

// Authenticate checks a submitted credential and returns the identity it names,
// with the principal for cookie.
func (s *Service) Authenticate(ctx context.Context, submittedEmail, submittedPasswd string) (Identity, Principal, error) {
	email := NormalizeEmail(submittedEmail)

	if !s.throttle.Allow(email) {
		return Identity{}, Principal{}, rejected()
	}

	user, err := s.users.ByEmail(ctx, email)
	switch {
	case errors.Is(err, ErrNoUser):
		// An unknown email verifies against a fixed hash, so it costs the same time.
		user = User{PasswdHash: password.Dummy()}
	case err != nil:
		return Identity{}, Principal{}, fmt.Errorf("%w: loading user by email: %w", ErrNotResolved, err)
	}

	ok, needsRehash, err := password.Verify(user.PasswdHash, submittedPasswd)
	if err != nil {
		return Identity{}, Principal{}, fmt.Errorf("%w: verifying the password: %w", ErrNotResolved, err)
	}

	if !ok || !user.Active {
		s.throttle.Fail(email)
		return Identity{}, Principal{}, rejected()
	}
	s.throttle.Reset(email)

	if needsRehash {
		s.rehash(ctx, user, submittedPasswd)
	}

	return user.Identity(), user.Principal(), nil
}

// Create writes a new user. It returns the row it wrote.
func (s *Service) Create(ctx context.Context, in NewUser) (User, error) {
	user, err := in.user(s.ids.New())
	if err != nil {
		return User{}, err
	}

	err = s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, user); err != nil {
			return err
		}
		return s.trail.Record(ctx, s.actor(ctx, user.ID), userChange(user.ID, audit.ActionCreated, "", "", ""))
	})
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// Invite creates a user with a generated first password, and hands the password
// back in plain text.
func (s *Service) Invite(ctx context.Context, in NewUser) (User, string, error) {
	in.Passwd = password.Generate()

	user, err := s.Create(ctx, in)
	if err != nil {
		return User{}, "", err
	}
	return user, in.Passwd, nil
}

// NewUser is the arguments for Create. Passwd is the first password, in plain text.
type NewUser struct {
	Email     string
	FirstName string
	LastName  string
	Role      Role
	Passwd    string
}

// user checks the submitted values and turns them into the User struct.
func (in NewUser) user(id ids.ID) (User, error) {
	var v validate.Validator
	v.Required(FieldEmail, in.Email)
	v.Required(FieldFirstName, in.FirstName)
	v.Required(FieldLastName, in.LastName)
	v.Check(in.Role.Valid(), FieldRole, validate.NotAllowed)
	if err := password.Check(FieldPasswd, in.Passwd); err != nil {
		v.Merge(err)
	}
	if err := v.Err(); err != nil {
		return User{}, err
	}

	hash, err := password.Hash(in.Passwd)
	if err != nil {
		return User{}, fmt.Errorf("auth: hashing the first password: %w", err)
	}

	user := User{
		ID:            id,
		Email:         NormalizeEmail(in.Email),
		FirstName:     strings.TrimSpace(in.FirstName),
		LastName:      strings.TrimSpace(in.LastName),
		PasswdHash:    hash,
		Role:          in.Role,
		Active:        true,
		SessionEpoch:  1,
		PasswdExpired: true,
	}

	return user, nil
}

// ChangePassword replaces the password of userID, and ends every session
// by incrementing SessionEpoch.
func (s *Service) ChangePassword(ctx context.Context, userID ids.ID, current, next string) (Principal, error) {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return Principal{}, fmt.Errorf("auth: loading user %s: %w", userID, err)
	}

	ok, _, err := password.Verify(user.PasswdHash, current)
	if err != nil {
		return Principal{}, fmt.Errorf("auth: verifying the current password: %w", err)
	}
	if !ok {
		return Principal{}, validate.Fail(FieldCurrentPass, validate.Incorrect)
	}

	if err := password.Check(FieldNewPass, next); err != nil {
		return Principal{}, err
	}

	hash, err := password.Hash(next)
	if err != nil {
		return Principal{}, fmt.Errorf("auth: hashing the new password: %w", err)
	}

	user.PasswdHash = hash
	user.PasswdExpired = false
	user.SessionEpoch++

	change := userChange(user.ID, audit.ActionPasswdChanged, FieldPasswd, "", audit.Marker)
	if err := s.write(ctx, user, change); err != nil {
		return Principal{}, err
	}

	s.throttle.Reset(user.Email)
	return user.Principal(), nil
}

// UpdateParams is what the users section changes on a user who already exists.
type UpdateParams struct {
	FirstName string
	LastName  string
	Role      Role
	Active    bool
}

// Update writes the edited fields and records one trail entry per field that
// moved. It writes nothing at all when nothing moved.
//
// Deactivating bumps the epoch, which ends every session of that user on the
// next request. A rename and a role change leave the sessions alone.
func (s *Service) Update(ctx context.Context, userID ids.ID, in UpdateParams) error {
	first := strings.TrimSpace(in.FirstName)
	last := strings.TrimSpace(in.LastName)

	var v validate.Validator
	v.Required(FieldFirstName, first)
	v.Required(FieldLastName, last)
	v.Check(in.Role.Valid(), FieldRole, validate.NotAllowed)
	if err := v.Err(); err != nil {
		return err
	}

	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("auth: loading user %s: %w", userID, err)
	}

	if s.locksOut(ctx, user, in) {
		return fmt.Errorf("%w: %s", ErrSelfLockout, user.ID)
	}

	var changes []audit.Change
	if first != user.FirstName {
		changes = append(changes, userChange(user.ID, audit.ActionNameChanged, FieldFirstName, user.FirstName, first))
		user.FirstName = first
	}
	if last != user.LastName {
		changes = append(changes, userChange(user.ID, audit.ActionNameChanged, FieldLastName, user.LastName, last))
		user.LastName = last
	}
	if in.Role != user.Role {
		changes = append(changes, userChange(user.ID, audit.ActionRoleChanged, FieldRole, string(user.Role), string(in.Role)))
		user.Role = in.Role
	}
	if in.Active != user.Active {
		action := audit.ActionReactivated
		if !in.Active {
			action = audit.ActionDeactivated
			user.SessionEpoch++
		}
		changes = append(changes, userChange(user.ID, action, FieldActive,
			strconv.FormatBool(user.Active), strconv.FormatBool(in.Active)))
		user.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	return s.write(ctx, user, changes...)
}

// locksOut reports whether the edit takes the section away from the very user
// making it: their own deactivation, or their own root role.
func (s *Service) locksOut(ctx context.Context, user User, in UpdateParams) bool {
	actor, ok := FromContext(ctx)
	if !ok || actor.UserID != user.ID {
		return false
	}
	return !in.Active || (user.Role == RootRole && in.Role != RootRole)
}

// ResetPassword gives a user a new first password and hands it back in plain text.
func (s *Service) ResetPassword(ctx context.Context, userID ids.ID) (string, error) {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("auth: loading user %s: %w", userID, err)
	}

	plain := password.Generate()
	hash, err := password.Hash(plain)
	if err != nil {
		return "", fmt.Errorf("auth: hashing the new first password: %w", err)
	}

	user.PasswdHash = hash
	user.PasswdExpired = true
	user.SessionEpoch++

	change := userChange(user.ID, audit.ActionPasswdReset, FieldPasswd, "", audit.Marker)
	if err := s.write(ctx, user, change); err != nil {
		return "", err
	}

	s.throttle.Reset(user.Email)
	return plain, nil
}

// List returns the users the section shows, ordered by email. It keeps the
// users that match query, and every user when query is empty.
func (s *Service) List(ctx context.Context, query string) ([]Account, error) {
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: listing users: %w", err)
	}

	terms := strings.Fields(strings.ToLower(query))

	accounts := make([]Account, 0, len(users))
	for _, user := range users {
		if matches(user, terms) {
			accounts = append(accounts, user.Account())
		}
	}
	return accounts, nil
}

// matches reports whether the user answers every term. A term answers when it
// is part of the address, of the first name or of the last name. Every term
// must match, so "ada lov" finds Ada Lovelace and "ada hop" finds nobody.
func matches(user User, terms []string) bool {
	fields := []string{
		strings.ToLower(user.Email),
		strings.ToLower(user.FirstName),
		strings.ToLower(user.LastName),
	}

	for _, term := range terms {
		found := false
		for _, field := range fields {
			if strings.Contains(field, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Account returns one user.
func (s *Service) Account(ctx context.Context, userID ids.ID) (Account, error) {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return Account{}, fmt.Errorf("auth: loading user %s: %w", userID, err)
	}
	return user.Account(), nil
}

// userChange is one stated fact about a user row.
func userChange(userID ids.ID, action, field, old, next string) audit.Change {
	change := audit.Change{
		Entity:   audit.EntityUser,
		EntityID: userID,
		Action:   action,
		FieldKey: field,
		Old:      old,
		New:      next,
	}
	return change
}

// write stores one row and records what changed, in one transaction. A failure
// on either side rolls both back, so the row and the trail agree.
func (s *Service) write(ctx context.Context, user User, changes ...audit.Change) error {
	err := s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.users.Update(ctx, user); err != nil {
			return err
		}
		return s.trail.Record(ctx, s.actor(ctx, user.ID), changes...)
	})
	return err
}

// actor is the user the trail holds responsible: whoever the request is
// authenticated as, and otherwise the user the write touches.
//
// The fallback covers the work that runs under no session. e.g. `pilam user add`
// creates the first user with nobody logged in, and that user is the only actor
// the entry can name.
func (s *Service) actor(ctx context.Context, fallback ids.ID) ids.ID {
	if identity, ok := FromContext(ctx); ok {
		return identity.UserID
	}
	return fallback
}

// rehash writes the password back under the current parameters.
func (s *Service) rehash(ctx context.Context, user User, submittedPasswd string) {
	logger := logging.FromContext(ctx)

	hash, err := password.Hash(submittedPasswd)
	if err != nil {
		logger.Warn("can't rehash the password", "user", user.ID, "err", err)
		return
	}

	user.PasswdHash = hash
	if err := s.write(ctx, user); err != nil {
		logger.Warn("can't store the rehashed password", "user", user.ID, "err", err)
	}
}

// NormalizeEmail is the one form an address is stored and looked up under.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func rejected() error { return validate.Fail("", validate.Incorrect) }

type contextKey struct{}

// NewContext returns a context carrying identity. The identity middleware puts
// it there once per request.
func NewContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// FromContext returns the identity ctx was resolved to, and whether the request
// is authenticated at all. An anonymous request yields the zero Identity and false.
func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	if !ok || identity.IsZero() {
		return Identity{}, false
	}
	return identity, true
}
