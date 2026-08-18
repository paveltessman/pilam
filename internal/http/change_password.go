package http

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/session"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const changePasswordPath = "/password"

func showChangePassword() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.FromContext(r.Context())
		if !ok {
			middleware.SendTo(w, r, loginPath)
			return
		}
		render(w, r, http.StatusOK, views.ChangePassword(views.ChangePasswordForm{Expired: identity.PasswdExpired}))
	}
	return handler
}

// submitChangePassword replaces the password of the user who is logged in.
//
// The change ends every session of that user, including this one, because the
// service bumps the session epoch. The device that made the change gets a new
// cookie here, so only the other devices lose their session.
func submitChangePassword(sessions *session.Manager, authSvc *auth.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		identity, ok := auth.FromContext(ctx)
		if !ok {
			middleware.SendTo(w, r, loginPath)
			return
		}

		current := r.PostFormValue(views.FieldCurrentPasswd)
		next := r.PostFormValue(views.FieldNewPasswd)
		repeat := r.PostFormValue(views.FieldRepeatPasswd)

		var v validate.Validator
		v.Required(views.FieldCurrentPasswd, current)
		v.Required(views.FieldNewPasswd, next)
		if v.Required(views.FieldRepeatPasswd, repeat) {
			v.Check(next == repeat, views.FieldRepeatPasswd, validate.Mismatch)
		}

		err := v.Err()
		var principal auth.Principal
		if err == nil {
			principal, err = authSvc.ChangePassword(ctx, identity.UserID, current, next)
		}
		if err != nil {
			errs, ok := validate.From(err)
			if !ok {
				logger.Error("the password change failed unexpectedly", "err", err)
				writeServerError(w)
				return
			}

			logger.Info("password change rejected", "reason", errs)
			form := views.ChangePasswordForm{Expired: identity.PasswdExpired, Errors: errs}
			render(w, r, http.StatusUnprocessableEntity, views.ChangePassword(form))
			return
		}

		middleware.SetSession(w, r, sessions, principal.String())
		logger.Info("password changed")
		http.Redirect(w, r, successPath, http.StatusSeeOther)
	}
	return handler
}
