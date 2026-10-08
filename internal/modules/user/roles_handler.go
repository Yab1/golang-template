package user

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

var (
	errAdminRoleProtected = errors.New("admin role cannot be renamed or deleted")
	errRoleInUse          = errors.New("role is assigned to one or more users")
	errUnauthorized       = errors.New("unauthorized")
)

type CreateRolePayload struct {
	Name        string `json:"name" validate:"required,max=255"`
	Level       int    `json:"level" validate:"required,min=0,max=1000"`
	Description string `json:"description" validate:"omitempty,max=4000"`
}

type UpdateRolePayload struct {
	Name        *string `json:"name" validate:"omitempty,max=255"`
	Level       *int    `json:"level" validate:"omitempty,min=0,max=1000"`
	Description *string `json:"description" validate:"omitempty,max=4000"`
}

type AssignRolePayload struct {
	Role string `json:"role" validate:"required,max=255"`
}

// getRoleHandler godoc
//
//	@Summary		Get role
//	@Description	Returns a role by id. Requires auth.
//	@Tags			roles
//	@Produce		json
//	@Param			roleID	path		int	true	"Role ID"
//	@Success		200		{object}	httpx.ObjectResponse{result=Role}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/roles/{roleID} [get]
func (m *Module) getRoleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "roleID"), 10, 64)
	if err != nil || id < 1 {
		m.respond.BadRequest(w, r, errors.New("invalid role id"))
		return
	}

	role, err := m.roles.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.NotFound(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := httpx.JSONResponse(w, http.StatusOK, role); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// createRoleHandler godoc
//
//	@Summary		Create role
//	@Description	Admin creates a new role (name, level, description).
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		CreateRolePayload	true	"Role payload"
//	@Success		201		{object}	httpx.ObjectResponse{result=Role}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/roles [post]
func (m *Module) createRoleHandler(w http.ResponseWriter, r *http.Request) {
	var payload CreateRolePayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	role := &Role{
		Name:        payload.Name,
		Level:       payload.Level,
		Description: payload.Description,
	}
	if err := m.roles.Create(r.Context(), role); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			m.respond.Conflict(w, r, errors.New("role name already exists"))
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionCreate, "role", strconv.FormatInt(role.ID, 10), map[string]any{
		"name":  role.Name,
		"level": role.Level,
	})

	if err := httpx.JSONResponse(w, http.StatusCreated, role); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// updateRoleHandler godoc
//
//	@Summary		Update role
//	@Description	Admin updates role name, level, and/or description. The seeded admin role cannot be renamed.
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			roleID	path		int					true	"Role ID"
//	@Param			payload	body		UpdateRolePayload	true	"Role patch"
//	@Success		200		{object}	httpx.ObjectResponse{result=Role}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/roles/{roleID} [patch]
func (m *Module) updateRoleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "roleID"), 10, 64)
	if err != nil || id < 1 {
		m.respond.BadRequest(w, r, errors.New("invalid role id"))
		return
	}

	var payload UpdateRolePayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	role, err := m.roles.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.NotFound(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	if role.Name == RoleAdmin && payload.Name != nil && *payload.Name != RoleAdmin {
		m.respond.BadRequest(w, r, errAdminRoleProtected)
		return
	}

	if payload.Name != nil {
		role.Name = *payload.Name
	}
	if payload.Level != nil {
		role.Level = *payload.Level
	}
	if payload.Description != nil {
		role.Description = *payload.Description
	}

	if err := m.roles.Update(r.Context(), role); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			m.respond.NotFound(w, r, err)
		case errors.Is(err, storage.ErrConflict):
			m.respond.Conflict(w, r, errors.New("role name already exists"))
		default:
			m.respond.InternalServerError(w, r, err)
		}
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionUpdate, "role", strconv.FormatInt(role.ID, 10), map[string]any{
		"name":  role.Name,
		"level": role.Level,
	})

	if err := httpx.JSONResponse(w, http.StatusOK, role); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// deleteRoleHandler godoc
//
//	@Summary		Delete role
//	@Description	Admin deletes a role. Fails if users are assigned or if the role is the seeded admin role.
//	@Tags			roles
//	@Produce		json
//	@Param			roleID	path	int	true	"Role ID"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Failure		409	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/roles/{roleID} [delete]
func (m *Module) deleteRoleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "roleID"), 10, 64)
	if err != nil || id < 1 {
		m.respond.BadRequest(w, r, errors.New("invalid role id"))
		return
	}

	role, err := m.roles.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.NotFound(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	if role.Name == RoleAdmin {
		m.respond.BadRequest(w, r, errAdminRoleProtected)
		return
	}

	n, err := m.roles.CountUsers(r.Context(), id)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if n > 0 {
		m.respond.Conflict(w, r, errRoleInUse)
		return
	}

	if err := m.roles.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.NotFound(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionDelete, "role", strconv.FormatInt(id, 10), map[string]any{
		"name": role.Name,
	})

	w.WriteHeader(http.StatusNoContent)
}

// assignUserRoleHandler godoc
//
//	@Summary		Assign role to user
//	@Description	Admin sets a user's role by name. Bumps token_version so existing access tokens die.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			userID	path		string				true	"User UUID"
//	@Param			payload	body		AssignRolePayload	true	"Role name"
//	@Success		200		{object}	httpx.ObjectResponse{result=User}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/users/{userID}/role [patch]
func (m *Module) assignUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		m.respond.BadRequest(w, r, errors.New("invalid user id"))
		return
	}

	var payload AssignRolePayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errUnauthorized)
		return
	}
	updatedBy := principal.ID

	u, err := m.AssignRole(r.Context(), userID, payload.Role, &updatedBy)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.NotFound(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionUpdate, "user", u.ID.String(), map[string]any{
		"role": payload.Role,
	})

	if err := httpx.JSONResponse(w, http.StatusOK, u); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// AssignRole resolves role by name and updates the user. Used by HTTP handlers
// and by the practitioner adapter.
func (m *Module) AssignRole(ctx context.Context, userID uuid.UUID, roleName string, updatedBy *uuid.UUID) (*User, error) {
	role, err := m.roles.GetByName(ctx, roleName)
	if err != nil {
		return nil, err
	}
	return m.users.UpdateRole(ctx, userID, role.ID, updatedBy)
}
