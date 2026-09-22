package user

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/mailer"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type CreateUserPayload struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Username string `json:"username" validate:"required,max=255"`
	Password string `json:"password" validate:"required,min=3,max=72"`
}

// createUserHandler godoc
//
//	@Summary		Register user
//	@Description	Create a user with the default "user" role. When AUTH_EMAIL_VERIFY_REQUIRED=true the account stays inactive until email verification.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		CreateUserPayload	true	"User payload"
//	@Success		201		{object}	httpx.ObjectResponse{result=User}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/users [post]
func (m *Module) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var payload CreateUserPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	u := &User{
		Email:     payload.Email,
		Username:  payload.Username,
		IsActive:  !m.verifyRequired,
		IsVisible: true,
		Role: Role{
			Name: "user",
		},
	}

	if err := u.Password.Set(payload.Password); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := m.users.Create(r.Context(), u); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			m.respond.Conflict(w, r, errors.New("email or username already exists"))
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionCreate, "user", u.ID.String(), map[string]any{
		"reference_id": u.ReferenceID,
	})

	if m.verifyRequired {
		if err := m.issueEmailVerify(r.Context(), u); err != nil {
			m.respond.Logger.Warnw("verification email failed", "email", u.Email, "error", err)
		}
	} else {
		m.sendWelcome(r, u)
	}

	if err := httpx.JSONResponse(w, http.StatusCreated, u); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

func (m *Module) sendWelcome(r *http.Request, u *User) {
	if m.mail == nil {
		return
	}

	name := m.appName
	if name == "" {
		name = "app"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	err := m.mail.Send(ctx, mailer.Message{
		To:      u.Email,
		Subject: fmt.Sprintf("Welcome to %s", name),
		Text:    fmt.Sprintf("Hi %s,\n\nYour account on %s is ready.\n", u.Username, name),
		HTML:    fmt.Sprintf("<p>Hi %s,</p><p>Your account on %s is ready.</p>", html.EscapeString(u.Username), html.EscapeString(name)),
	})
	if err != nil {
		m.respond.Logger.Warnw("welcome email failed", "email", u.Email, "error", err)
	}
}
