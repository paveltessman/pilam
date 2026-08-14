// Package ids issues the system-generated identifiers every entity is keyed by.
//
// Generation sits behind an interface for the consistent testing and demo seeding.
//
// Production uses NewGenerator, tests and the seed uses NewDeterministic.
package ids

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type ID = uuid.UUID

var Nil = uuid.Nil

var ErrInvalid = errors.New("invalid identifier")

// canonicalLength is 8-4-4-4-12 hex digits plus the four hyphens.
const canonicalLength = 36

// Generator issues new identifiers. Services take one rather than calling a
// package function, which is what lets the seed substitute the deterministic
// implementation for the random one.
type Generator interface {
	// New returns a fresh identifier. It never returns Nil and is safe for
	// concurrent use.
	New() ID
}

// NewGenerator returns the production generator: UUIDv7 over crypto/rand.
func NewGenerator() Generator { return randomGenerator{} }

type randomGenerator struct{}

func (randomGenerator) New() ID {
	id, err := uuid.NewV7()
	if err != nil {
		// The only failure path is crypto/rand failing, which the runtime
		// already treats as fatal. There is no identifier to return and no
		// caller that could do anything useful with the error, so it does not
		// become part of the interface.
		panic(fmt.Errorf("ids: generating a v7 identifier: %w", err))
	}
	return id
}

// Parse reads the canonical text form.
//
// The other variants (braced, urn:uuid:, unhyphenated) are rejected.
// Hex case is not rejected.
func Parse(s string) (ID, error) {
	if len(s) != canonicalLength {
		return Nil, fmt.Errorf("%w: %q is not the canonical 8-4-4-4-12 form", ErrInvalid, s)
	}

	id, err := uuid.Parse(s)
	if err != nil {
		return Nil, fmt.Errorf("%w: %q: %w", ErrInvalid, s, err)
	}

	if id == Nil {
		return Nil, fmt.Errorf("%w: %q is the nil identifier", ErrInvalid, s)
	}

	return id, nil
}

// MustParse is Parse for identifiers written into source: test fixtures, and
// any well-known row the seed pins to a fixed id. If it panics, that means the input
// contains a typo.
func MustParse(s string) ID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
