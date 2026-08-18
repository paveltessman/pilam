package auth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

const sep = ":"

var ErrUnknownSubject = errors.New("auth: unknown subject")

// Principal is what the session cookie carries.
type Principal struct {
	UserID ids.ID
	Epoch  int
}

// String returns the text for cookie, "<uuid>:<epoch>".
func (p Principal) String() string {
	return p.UserID.String() + sep + strconv.Itoa(p.Epoch)
}

// ParsePrincipal builds Principal from the cookie string.
func ParsePrincipal(subject string) (Principal, error) {
	rawID, rawEpoch, found := strings.Cut(subject, sep)
	if !found {
		return Principal{}, fmt.Errorf("%w: %q holds no %q", ErrUnknownSubject, subject, sep)
	}

	id, err := ids.Parse(rawID)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: can't parse the id: %w", ErrUnknownSubject, err)
	}

	epoch, err := strconv.Atoi(rawEpoch)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: can't parse the epoch: %w", ErrUnknownSubject, err)
	}

	return Principal{UserID: id, Epoch: epoch}, nil
}
