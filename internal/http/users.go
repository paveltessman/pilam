package http

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const (
	usersPath    = "/users"
	userNewPath  = usersPath + "/new"
	userPath     = usersPath + "/{id}"
	userPassPath = userPath + "/password"
)

// showUsers lists the users, active and inactive. The "q" parameter searches
// the address, the first name and the last name. An empty "q" lists everybody.
//
// htmx asks for the same URL as the user types. Such a request gets the table
// alone, because the rest of the screen already stands in the browser.
func showUsers(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		query := r.URL.Query().Get(views.FieldUserSearch)

		accounts, err := authSvc.List(ctx, query)
		if err != nil {
			logging.FromContext(ctx).Error("listing users failed", "err", err)
			writeServerError(w)
			return
		}

		page := views.UsersPage{Chrome: chrome(ctx), Users: accounts, Query: query}
		if middleware.IsFragment(ctx) {
			render(w, r, http.StatusOK, views.UsersResults(page))
			return
		}
		render(w, r, http.StatusOK, views.Users(page))
	}
	return handler
}

// showNewUser renders the empty create form.
func showNewUser() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.UserForm{Chrome: chrome(r.Context()), Role: auth.MemberRole, Active: true}
		render(w, r, http.StatusOK, views.User(form))
	}
	return handler
}

// createUser writes the user and shows the generated first password once.
func createUser(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedUser(r)
		form.Chrome = chrome(ctx)

		user, passwd, err := authSvc.Invite(ctx, auth.NewUser{
			Email:     form.Email,
			FirstName: form.FirstName,
			LastName:  form.LastName,
			Role:      form.Role,
		})
		if errors.Is(err, auth.ErrEmailTaken) {
			err = validate.Fail(views.FieldEmail, validate.Taken)
		}
		if err != nil {
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("creating a user failed unexpectedly", "err", err)
				writeServerError(w)
				return
			}

			logger.Info("user create rejected", "reason", errs)
			form.Errors = errs
			render(w, r, http.StatusUnprocessableEntity, views.User(form))
			return
		}

		logger.Info("user created", "user", user.ID, "email", user.Email)
		showPasswdOnce(w, r, labels.UsersPasswdCreated, user.Account(), passwd)
	}
	return handler
}

// showUser renders the edit form for one user, and the audit trail of that user
// under it.
func showUser(authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		account, ok := loadUser(w, r, authSvc)
		if !ok {
			return
		}

		trail, ok := loadTrail(w, r, authSvc, log, account.ID)
		if !ok {
			return
		}

		form := views.UserForm{
			Chrome:    chrome(ctx),
			UserID:    account.ID.String(),
			Email:     account.Email,
			FirstName: account.FirstName,
			LastName:  account.LastName,
			Role:      account.Role,
			Active:    account.Active,
			Trail:     trail,
		}
		if isSaved(r) {
			form.Notice = labels.Saved
		}
		render(w, r, http.StatusOK, views.User(form))
	}
	return handler
}

// saveUser writes the name, the role and the active flag.
func saveUser(authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		account, ok := loadUser(w, r, authSvc)
		if !ok {
			return
		}

		form := submittedUser(r)
		form.Chrome = chrome(ctx)
		form.UserID = account.ID.String()
		form.Email = account.Email

		err := authSvc.Update(ctx, account.ID, auth.UpdateParams{
			FirstName: form.FirstName,
			LastName:  form.LastName,
			Role:      form.Role,
			Active:    form.Active,
		})
		switch {
		case err == nil:
			logger.Info("user updated", "user", account.ID)
			redirectSaved(w, r, usersPath+"/"+form.UserID)
			return

		case errors.Is(err, auth.ErrSelfLockout):
			logger.Info("user update refused: self lockout", "user", account.ID)
			form.Alert = labels.UsersSelfLockout

		default:
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("updating a user failed unexpectedly", "user", account.ID, "err", err)
				writeServerError(w)
				return
			}
			logger.Info("user update rejected", "user", account.ID, "reason", errs)
			form.Errors = errs
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := loadTrail(w, r, authSvc, log, account.ID)
		if !ok {
			return
		}
		form.Trail = trail

		render(w, r, http.StatusUnprocessableEntity, views.User(form))
	}
	return handler
}

// resetUserPasswd gives the user a new first password and shows it once. It
// ends every session of that user, and holds them on the change screen at the
// next login.
func resetUserPasswd(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		account, ok := loadUser(w, r, authSvc)
		if !ok {
			return
		}

		passwd, err := authSvc.ResetPassword(ctx, account.ID)
		if err != nil {
			logger.Error("resetting a password failed", "user", account.ID, "err", err)
			writeServerError(w)
			return
		}

		logger.Info("user password reset", "user", account.ID)
		showPasswdOnce(w, r, labels.UsersPasswdReset, account, passwd)
	}
	return handler
}

// showPasswdOnce renders a first password.
func showPasswdOnce(w http.ResponseWriter, r *http.Request, title string, account auth.Account, passwd string) {
	page := views.UserPasswdPage{
		Chrome: chrome(r.Context()),
		Title:  title,
		Email:  account.Email,
		Name:   labels.Name(account.FirstName, account.LastName),
		Passwd: passwd,
	}
	render(w, r, http.StatusOK, views.UserPasswd(page))
}

// loadUser reads the {id} of the route and returns the user it names. It
// answers 404 itself for an unreadable id and for a user who is not there, and
// then reports false.
func loadUser(w http.ResponseWriter, r *http.Request, authSvc *auth.Service) (auth.Account, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable user id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return auth.Account{}, false
	}

	account, err := authSvc.Account(ctx, id)
	if errors.Is(err, auth.ErrNoUser) {
		logger.Info("no such user", "user", id)
		http.NotFound(w, r)
		return auth.Account{}, false
	}
	if err != nil {
		logger.Error("loading a user failed", "user", id, "err", err)
		writeServerError(w)
		return auth.Account{}, false
	}

	return account, true
}

// loadTrail reads the audit trail of one user and names the actor of every
// entry. It answers 500 itself and reports false when the trail cannot be read.
func loadTrail(w http.ResponseWriter, r *http.Request, authSvc *auth.Service, log *audit.Log, userID ids.ID) ([]views.UserEvent, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	entries, err := log.Entity(ctx, audit.EntityUser, userID, audit.DefaultLimit)
	if err != nil {
		logger.Error("loading the audit trail failed", "user", userID, "err", err)
		writeServerError(w)
		return nil, false
	}

	actorIDs := make([]ids.ID, len(entries))
	for i, entry := range entries {
		actorIDs[i] = entry.ActorID
	}

	actors, err := authSvc.Actors(ctx, actorIDs...)
	if err != nil {
		logger.Error("naming the actors of the audit trail failed", "user", userID, "err", err)
		writeServerError(w)
		return nil, false
	}

	events := make([]views.UserEvent, len(entries))
	for i, entry := range entries {
		events[i] = views.UserEvent{Entry: entry, Actor: actors[entry.ActorID]}
	}
	return events, true
}

// submittedUser reads the fields the create and the edit form post. The service
// checks the values: this only carries them.
func submittedUser(r *http.Request) views.UserForm {
	form := views.UserForm{
		Email:     r.PostFormValue(views.FieldEmail),
		FirstName: r.PostFormValue(views.FieldUserFirstName),
		LastName:  r.PostFormValue(views.FieldUserLastName),
		Role:      auth.Role(r.PostFormValue(views.FieldUserRole)),
		// An unchecked box posts nothing, which is what deactivates a user.
		Active: r.PostFormValue(views.FieldUserActive) != "",
	}
	return form
}
