// validate.go — the rules for emails, passwords, roles and ids, and password hashing.
package main

import (
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// minPasswordLength follows NIST's current guidance for a password that is the only factor.
	// Length is the only rule: no forced capitals, digits or symbols, which push people towards
	// guessable patterns such as "Password1!".
	minPasswordLength = 15 // characters

	// maxPasswordBytes is where bcrypt stops reading. A longer password is refused rather than
	// silently cut short. 72 bytes is 72 English characters, or about 24 Thai ones.
	maxPasswordBytes = 72

	// bcryptCost makes each hash take a few hundred milliseconds, which is what makes guessing
	// passwords from a stolen copy of the table slow.
	bcryptCost = 12
)

// roles are the two roles a member can hold (FR-0.3).
var roles = []string{"admin", "recruiter"}

// normaliseEmail trims and lower-cases an email, so that Ploy@Acme.example and ploy@acme.example
// are the same member. It returns the email, or a problem to report.
func normaliseEmail(raw string) (string, string) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", "email is required"
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", "email must be a plain address such as ploy@acme.example"
	}
	return email, ""
}

// checkPassword returns a problem to report, or "" when the password is acceptable.
func checkPassword(password string) string {
	switch {
	case utf8.RuneCountInString(password) < minPasswordLength:
		return fmt.Sprintf("password must be at least %d characters", minPasswordLength)
	case len(password) > maxPasswordBytes:
		return fmt.Sprintf("password must be at most %d bytes", maxPasswordBytes)
	}
	return ""
}

// checkRole returns a problem to report, or "" when role is admin or recruiter.
func checkRole(role string) string {
	if !slices.Contains(roles, role) {
		return "role must be admin or recruiter"
	}
	return ""
}

// isUUID reports whether s has the shape of a UUID, such as 00000000-0000-4000-8000-000000000001.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

// hashPassword returns the bcrypt hash that is stored instead of the password.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(hash), err
}
