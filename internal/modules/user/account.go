package user

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/mailer"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type TokenPayload struct {
	Token string `json:"token" validate:"required"`
}

type EmailPayload struct {
	Email string `json:"email" validate:"required,email,max=255"`
}

type ResetPasswordPayload struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=3,max=72"`
}

type ChangePasswordPayload struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=3,max=72"`
}

// verifyEmailHandler godoc
//
//	@Summary		Verify email
//	@Description	Consume email verification token and activate the account
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	TokenPayload	true	"Verification token"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/authentication/verify-email [post]
func (m *Module) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var payload TokenPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	tok, err := m.tokens.Consume(r.Context(), payload.Token, PurposeEmailVerify)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.BadRequest(w, r, errors.New("invalid or expired token"))
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := m.users.Activate(r.Context(), tok.UserID); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// resendVerificationHandler godoc
//
//	@Summary		Resend verification email
//	@Description	Send a new email verification token. Always 204 to avoid account enumeration.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	EmailPayload	true	"Email"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/authentication/resend-verification [post]
func (m *Module) resendVerificationHandler(w http.ResponseWriter, r *http.Request) {
	var payload EmailPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	u, err := m.users.GetByEmail(r.Context(), payload.Email)
	if err == nil && !u.IsActive {
		if err := m.issueEmailVerify(r.Context(), u); err != nil {
			m.respond.InternalServerError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// forgotPasswordHandler godoc
//
//	@Summary		Forgot password
//	@Description	Email a password-reset token if the account exists. Always 204.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	EmailPayload	true	"Email"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/authentication/forgot-password [post]
func (m *Module) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var payload EmailPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	u, err := m.users.GetByEmail(r.Context(), payload.Email)
	if err == nil {
		if err := m.issuePasswordReset(r.Context(), u); err != nil {
			m.respond.InternalServerError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// resetPasswordHandler godoc
//
//	@Summary		Reset password
//	@Description	Consume reset token, set new password, revoke all sessions
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	ResetPasswordPayload	true	"Token + new password"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/authentication/reset-password [post]
func (m *Module) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var payload ResetPasswordPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	tok, err := m.tokens.Consume(r.Context(), payload.Token, PurposePasswordReset)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.BadRequest(w, r, errors.New("invalid or expired token"))
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	var pw password
	if err := pw.Set(payload.Password); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if err := m.users.UpdatePassword(r.Context(), tok.UserID, pw.hash); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	_ = m.refresh.RevokeAllForUser(r.Context(), tok.UserID)
	_, _ = m.users.IncrementTokenVersion(r.Context(), tok.UserID)
	// Activate on reset so forgot-password works for unverified accounts that proved email ownership.
	_ = m.users.Activate(r.Context(), tok.UserID)

	w.WriteHeader(http.StatusNoContent)
}

// changePasswordHandler godoc
//
//	@Summary		Change password
//	@Description	Authenticated password change. Revokes all refresh sessions.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	ChangePasswordPayload	true	"Current + new password"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/authentication/change-password [post]
func (m *Module) changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var payload ChangePasswordPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if payload.CurrentPassword == payload.NewPassword {
		m.respond.BadRequest(w, r, errors.New("new password must differ from current password"))
		return
	}

	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("unauthorized"))
		return
	}

	u, err := m.users.GetByID(r.Context(), principal.ID)
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}
	if err := u.Password.Compare(payload.CurrentPassword); err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}

	var pw password
	if err := pw.Set(payload.NewPassword); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if err := m.users.UpdatePassword(r.Context(), u.ID, pw.hash); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	_ = m.refresh.RevokeAllForUser(r.Context(), u.ID)
	_, _ = m.users.IncrementTokenVersion(r.Context(), u.ID)
	if principal.AccessJTI != "" {
		_ = m.blocklist.Ban(r.Context(), principal.AccessJTI, time.Unix(principal.AccessExp, 0))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *Module) issueEmailVerify(ctx context.Context, u *User) error {
	plain, hash, err := NewPlainToken()
	if err != nil {
		return err
	}
	if err := m.tokens.InvalidateOpen(ctx, u.ID, PurposeEmailVerify); err != nil {
		return err
	}
	if err := m.tokens.Create(ctx, u.ID, PurposeEmailVerify, hash, time.Now().Add(m.verifyExp)); err != nil {
		return err
	}
	m.sendTokenMail(ctx, u, "Verify your email", "email verification", plain, "verify-email")
	return nil
}

func (m *Module) issuePasswordReset(ctx context.Context, u *User) error {
	plain, hash, err := NewPlainToken()
	if err != nil {
		return err
	}
	if err := m.tokens.InvalidateOpen(ctx, u.ID, PurposePasswordReset); err != nil {
		return err
	}
	if err := m.tokens.Create(ctx, u.ID, PurposePasswordReset, hash, time.Now().Add(m.resetExp)); err != nil {
		return err
	}
	m.sendTokenMail(ctx, u, "Reset your password", "password reset", plain, "reset-password")
	return nil
}

func (m *Module) sendTokenMail(ctx context.Context, u *User, subject, kind, plain, routeHint string) {
	if m.mail == nil {
		return
	}
	name := m.appName
	if name == "" {
		name = "app"
	}
	base := strings.TrimRight(m.publicBaseURL, "/")
	hint := fmt.Sprintf("POST %s/api/v1/authentication/%s with {\"token\":\"...\"}", base, routeHint)

	mailCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	err := m.mail.Send(mailCtx, mailer.Message{
		To:      u.Email,
		Subject: fmt.Sprintf("%s — %s", name, subject),
		Text: fmt.Sprintf(
			"Hi %s,\n\nUse this %s token (expires soon):\n\n%s\n\n%s\n",
			u.Username, kind, plain, hint,
		),
		HTML: fmt.Sprintf(
			"<p>Hi %s,</p><p>Your %s token:</p><p><code>%s</code></p><p>%s</p>",
			html.EscapeString(u.Username),
			html.EscapeString(kind),
			html.EscapeString(plain),
			html.EscapeString(hint),
		),
	})
	if err != nil {
		m.respond.Logger.Warnw("token email failed", "kind", kind, "email", u.Email, "error", err)
	}
}
