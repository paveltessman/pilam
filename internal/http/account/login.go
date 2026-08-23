// Package account holds the screens a user reaches about their own account:
// signing in, signing out, and changing their password.
package account

import (
	"net/http"
	"strings"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/account/views"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/session"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Where a login lands, and where the root path sends the browser.
const successPath = paths.Models

func ShowLogin() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.FromContext(r.Context()); ok {
			http.Redirect(w, r, successPath, http.StatusSeeOther)
			return
		}
		shared.Render(w, r, http.StatusOK, views.Login(views.LoginForm{}))
	}
	return handler
}

// SubmitLogin checks the submitted credential and, if it holds, starts a session.
func SubmitLogin(sessions *session.Manager, authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := views.LoginForm{
			Email: strings.TrimSpace(r.PostFormValue(views.FieldEmail)),
		}
		submittedPassword := r.PostFormValue(views.FieldPassword)

		var v validate.Validator
		v.Required(views.FieldEmail, form.Email)
		v.Required(views.FieldPassword, submittedPassword)

		err := v.Err()
		var principal auth.Principal
		if err == nil {
			_, principal, err = authSvc.Authenticate(ctx, form.Email, submittedPassword)
		}
		if err != nil {
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("login failed unexpectedly", "err", err)
				shared.WriteServerError(w)
				return
			}

			logger.Info("login rejected", "email", form.Email, "reason", errs)
			form.Errors = errs

			shared.Render(w, r, http.StatusUnauthorized, views.Login(form))
			return
		}

		middleware.SetSession(w, r, sessions, principal.String())
		logging.FromContext(ctx).Info("login accepted", "email", form.Email)
		http.Redirect(w, r, successPath, http.StatusSeeOther)
	}
	return handler
}

// SubmitLogout drops the session.
func SubmitLogout() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		middleware.ClearSession(w, r)
		logging.FromContext(r.Context()).Info("logout")
		http.Redirect(w, r, paths.Login, http.StatusSeeOther)
	}
	return handler
}
