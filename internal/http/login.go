package http

import (
	"net/http"
	"strings"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/session"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const (
	loginPath   = "/login"
	logoutPath  = "/logout"
	successPath = "/"
)

func showLogin() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.FromContext(r.Context()); ok {
			http.Redirect(w, r, successPath, http.StatusSeeOther)
			return
		}
		render(w, r, http.StatusOK, views.Login(views.LoginForm{}))
	}
	return handler
}

// submitLogin checks the submitted credential and, if it holds, starts a session.
func submitLogin(sessions *session.Manager, authSvc *auth.Service) http.HandlerFunc {
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
				writeServerError(w)
				return
			}

			logger.Info("login rejected", "email", form.Email, "reason", errs)
			form.Errors = errs

			render(w, r, http.StatusUnauthorized, views.Login(form))
			return
		}

		middleware.SetSession(w, r, sessions, principal.String())
		logging.FromContext(ctx).Info("login accepted", "email", form.Email)
		http.Redirect(w, r, successPath, http.StatusSeeOther)
	}
	return handler
}

// submitLogout drops the session.
func submitLogout() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		middleware.ClearSession(w, r)
		logging.FromContext(r.Context()).Info("logout")
		http.Redirect(w, r, loginPath, http.StatusSeeOther)
	}
	return handler
}
