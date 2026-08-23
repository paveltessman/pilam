// Package users holds the users section: the list, the card, and the password
// reset. Only a root reaches any of it.
package users

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/http/users/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// ShowList lists the users, active and inactive. The "q" parameter searches
// the address, the first name and the last name. An empty "q" lists everybody.
//
// htmx asks for the same URL as the user types. Such a request gets the table
// alone, because the rest of the screen already stands in the browser.
func ShowList(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		query := r.URL.Query().Get(views.FieldUserSearch)

		accounts, err := authSvc.List(ctx, query)
		if err != nil {
			logging.FromContext(ctx).Error("listing users failed", "err", err)
			shared.WriteServerError(w)
			return
		}

		page := views.UsersPage{Chrome: shared.Chrome(ctx), Users: accounts, Query: query}
		if middleware.IsFragment(ctx) {
			shared.Render(w, r, http.StatusOK, views.UsersResults(page))
			return
		}
		shared.Render(w, r, http.StatusOK, views.Users(page))
	}
	return handler
}

// ShowNew renders the empty create form.
func ShowNew() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.UserForm{Chrome: shared.Chrome(r.Context()), Role: auth.MemberRole, Active: true}
		shared.Render(w, r, http.StatusOK, views.User(form))
	}
	return handler
}

// Create writes the user and shows the generated first password once.
func Create(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)

		user, passwd, err := authSvc.Invite(ctx, auth.NewUser{
			Email:     form.Email,
			FirstName: form.FirstName,
			LastName:  form.LastName,
			Role:      form.Role,
		})
		if errors.Is(err, auth.ErrEmailTaken) {
			err = validate.Fail(views.FieldUserEmail, validate.Taken)
		}
		if err != nil {
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("creating a user failed unexpectedly", "err", err)
				shared.WriteServerError(w)
				return
			}

			logger.Info("user create rejected", "reason", errs)
			form.Errors = errs
			shared.Render(w, r, http.StatusUnprocessableEntity, views.User(form))
			return
		}

		logger.Info("user created", "user", user.ID, "email", user.Email)
		showPasswdOnce(w, r, labels.UsersPasswdCreated, user.Account(), passwd)
	}
	return handler
}

// Show renders the edit form for one user, and the audit trail of that user
// under it.
func Show(authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		account, ok := loadOne(w, r, authSvc)
		if !ok {
			return
		}

		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityUser, account.ID)
		if !ok {
			return
		}

		form := views.UserForm{
			Chrome:    shared.Chrome(ctx),
			UserID:    account.ID.String(),
			Email:     account.Email,
			FirstName: account.FirstName,
			LastName:  account.LastName,
			Role:      account.Role,
			Active:    account.Active,
			Trail:     trail,
		}
		if shared.IsSaved(r) {
			form.Notice = labels.Saved
		}
		shared.Render(w, r, http.StatusOK, views.User(form))
	}
	return handler
}

// Save writes the name, the role and the active flag.
func Save(authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		account, ok := loadOne(w, r, authSvc)
		if !ok {
			return
		}

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)
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
			shared.RedirectSaved(w, r, paths.Users+"/"+form.UserID)
			return

		case errors.Is(err, auth.ErrSelfLockout):
			logger.Info("user update refused: self lockout", "user", account.ID)
			form.Alert = labels.UsersSelfLockout

		default:
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("updating a user failed unexpectedly", "user", account.ID, "err", err)
				shared.WriteServerError(w)
				return
			}
			logger.Info("user update rejected", "user", account.ID, "reason", errs)
			form.Errors = errs
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityUser, account.ID)
		if !ok {
			return
		}
		form.Trail = trail

		shared.Render(w, r, http.StatusUnprocessableEntity, views.User(form))
	}
	return handler
}

// ResetPasswd gives the user a new first password and shows it once. It
// ends every session of that user, and holds them on the change screen at the
// next login.
func ResetPasswd(authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		account, ok := loadOne(w, r, authSvc)
		if !ok {
			return
		}

		passwd, err := authSvc.ResetPassword(ctx, account.ID)
		if err != nil {
			logger.Error("resetting a password failed", "user", account.ID, "err", err)
			shared.WriteServerError(w)
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
		Chrome: shared.Chrome(r.Context()),
		Title:  title,
		Email:  account.Email,
		Name:   labels.Name(account.FirstName, account.LastName),
		Passwd: passwd,
	}
	shared.Render(w, r, http.StatusOK, views.UserPasswd(page))
}

// loadOne reads the {id} of the route and returns the user it names. It
// answers 404 itself for an unreadable id and for a user who is not there, and
// then reports false.
func loadOne(w http.ResponseWriter, r *http.Request, authSvc *auth.Service) (auth.Account, bool) {
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
		shared.WriteServerError(w)
		return auth.Account{}, false
	}

	return account, true
}

// submitted reads the fields the create and the edit form post. The service
// checks the values: this only carries them.
func submitted(r *http.Request) views.UserForm {
	form := views.UserForm{
		Email:     r.PostFormValue(views.FieldUserEmail),
		FirstName: r.PostFormValue(views.FieldUserFirstName),
		LastName:  r.PostFormValue(views.FieldUserLastName),
		Role:      auth.Role(r.PostFormValue(views.FieldUserRole)),
		// An unchecked box posts nothing, which is what deactivates a user.
		Active: r.PostFormValue(views.FieldUserActive) != "",
	}
	return form
}
