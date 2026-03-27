package util

import (
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// NewID generates a new ULID string (26 characters).
func NewID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
