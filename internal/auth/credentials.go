package auth

import (
	"context"
	"crypto/subtle"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Temp credentinals just for v1 demo. Will be removed as soon as we land
// a proper authentication system.
const (
	Username = "manager"
	Password = "password"
)

// Authenticate checks a submitted credential and returns the identity it names.
func Authenticate(_ context.Context, submittedUsername, submittedPassword string) (Identity, error) {
	nameOK := subtle.ConstantTimeCompare([]byte(submittedUsername), []byte(Username))
	passwordOK := subtle.ConstantTimeCompare([]byte(submittedPassword), []byte(Password))

	if nameOK&passwordOK != 1 {
		return Identity{}, validate.Fail("", validate.Incorrect)
	}
	return Identity{Subject: Subject, Role: RoleManager}, nil
}
