// members.go — the HTTP handlers for member CRUD (UC-6) and the readiness probe.
//
// Not yet protected: every member endpoint is open until signing in and the admin check are added.
// Run it only on your own machine until then.
package main

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// API holds what the handlers need.
type API struct {
	store *Store
}

// handleReady answers the readiness probe: the service can serve requests only while the database
// answers. Unlike the liveness probe, a failure here makes Kubernetes stop sending traffic rather
// than restart the process.
func (a *API) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// GET /members — the active members with their roles (FR-6.5).
func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := a.store.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

// GET /members/{id} — one member, including a removed one (FR-6.2).
func (a *API) getMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	member, err := a.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

type createMemberRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` // a temporary password the member must replace (FR-0.5)
	Role     string `json:"role"`
}

// POST /members — add a member with a temporary password (FR-6.1).
func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	var req createMemberRequest
	if !readJSON(w, r, &req) {
		return
	}

	email, emailProblem := normaliseEmail(req.Email)
	problems := nonEmpty(emailProblem, checkPassword(req.Password), checkRole(req.Role))
	if len(problems) > 0 {
		writeInvalid(w, problems)
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash the password", err)
		return
	}
	member, err := a.store.Create(r.Context(), email, hash, req.Role)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", "/members/"+member.ID)
	writeJSON(w, http.StatusCreated, member)
}

type changeRoleRequest struct {
	Role string `json:"role"`
}

// PATCH /members/{id} — change a member's role (FR-6.1). The role is the only field that can
// change; any other field in the body is refused.
func (a *API) changeMemberRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req changeRoleRequest
	if !readJSON(w, r, &req) {
		return
	}
	if problem := checkRole(req.Role); problem != "" {
		writeInvalid(w, []string{problem})
		return
	}
	member, err := a.store.ChangeRole(r.Context(), id, req.Role)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

type setPasswordRequest struct {
	Password string `json:"password"` // the new temporary password
}

// PUT /members/{id}/password — give a member a new temporary password (FR-6.4).
func (a *API) setMemberPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req setPasswordRequest
	if !readJSON(w, r, &req) {
		return
	}
	if problem := checkPassword(req.Password); problem != "" {
		writeInvalid(w, []string{problem})
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash the password", err)
		return
	}
	member, err := a.store.SetPassword(r.Context(), id, hash)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

// DELETE /members/{id} — remove a member (FR-6.1). The row stays, marked removed (FR-6.2).
func (a *API) removeMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.Remove(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pathID reads {id} from the URL. Anything that is not a UUID is a 400, not a 404.
func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "id must be a UUID", nil)
		return "", false
	}
	return id, true
}

// writeStoreError turns an error from the store into the matching HTTP status.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error(), nil)
	case errors.Is(err, ErrEmailTaken), errors.Is(err, ErrRemoved), errors.Is(err, ErrLastAdmin):
		writeError(w, http.StatusConflict, err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "database error", err)
	}
}

// nonEmpty returns the problems that are not "".
func nonEmpty(problems ...string) []string {
	var out []string
	for _, p := range problems {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
