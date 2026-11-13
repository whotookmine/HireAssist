// Package system holds the clock and the id generator.
package system

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Clock is the real clock, in UTC.
type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

// IDs makes random references. They carry no information about what they refer to, which is
// what lets an erasure's reference stand in for a person who is no longer there (FR-5.5).
type IDs struct{}

func (IDs) NewID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error (Go 1.24+)
	return prefix + "-" + hex.EncodeToString(b)
}
