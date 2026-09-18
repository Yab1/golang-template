package user

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/authn"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type CreateUserTokenPayload struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=3,max=72"`
}

type RefreshPayload struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutPayload struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// createTokenHandler godoc
//
//	@Summary		Login
//	@Description	Exchange email and password for access + refresh JWTs
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		CreateUserTokenPayload	true	"Credentials"
//	@Success		201		{object}	httpx.ObjectResponse{result=TokenResponse}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/authentication/token [post]
func (m *Module) createTokenHandler(w http.ResponseWriter, r *http.Request) {
	var payload CreateUserTokenPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	u, err := m.users.GetByEmail(r.Context(), payload.Email)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			m.respond.Unauthorized(w, r, err)
		default:
			m.respond.InternalServerError(w, r, err)
		}
		return
	}

	if err := u.Password.Compare(payload.Password); err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}

	if !u.IsActive {
		m.respond.Forbidden(w, r, errors.New("email not verified"))
		return
	}

	pair, err := m.issuePair(r.Context(), u)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := httpx.JSONResponse(w, http.StatusCreated, pair); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// refreshTokenHandler godoc
//
//	@Summary		Refresh tokens
//	@Description	Rotate refresh token and issue a new access token. Reuse of a revoked refresh token kills all sessions.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RefreshPayload	true	"Refresh token"
//	@Success		200		{object}	httpx.ObjectResponse{result=TokenResponse}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/authentication/refresh [post]
func (m *Module) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var payload RefreshPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	claims, err := m.parseRefresh(payload.RefreshToken)
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}

	jti, err := uuid.Parse(mustClaim(claims, "jti"))
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}
	userID, err := uuid.Parse(mustClaim(claims, "sub"))
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}

	blocked, err := m.blocklist.Blocked(r.Context(), jti.String())
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if blocked {
		m.respond.Unauthorized(w, r, errors.New("refresh token revoked"))
		return
	}

	u, err := m.users.GetByID(r.Context(), userID)
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}
	if ver, ok := authn.ClaimInt(claims, "ver"); ok && ver != u.TokenVersion {
		m.respond.Unauthorized(w, r, errors.New("refresh token revoked"))
		return
	}

	newJTI := uuid.New()
	expiresAt := time.Now().Add(m.refreshExp)
	err = m.refresh.Rotate(r.Context(), jti, newJTI, userID, expiresAt)
	if err != nil {
		if errors.Is(err, errRefreshReuse) {
			_ = m.refresh.RevokeAllForUser(r.Context(), userID)
			_, _ = m.users.IncrementTokenVersion(r.Context(), userID)
			m.respond.Unauthorized(w, r, errors.New("refresh token reuse detected"))
			return
		}
		if errors.Is(err, storage.ErrNotFound) {
			m.respond.Unauthorized(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
		return
	}

	_ = m.blocklist.Ban(r.Context(), jti.String(), time.Unix(int64FromExp(claims), 0))

	pair, err := m.issuePairWithRefreshJTI(r.Context(), u, newJTI, expiresAt)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := httpx.JSONResponse(w, http.StatusOK, pair); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// logoutHandler godoc
//
//	@Summary		Logout
//	@Description	Revoke the refresh token and blacklist the current access token jti
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body	LogoutPayload	true	"Refresh token"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/authentication/logout [post]
func (m *Module) logoutHandler(w http.ResponseWriter, r *http.Request) {
	var payload LogoutPayload
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
		m.respond.Unauthorized(w, r, errors.New("unauthorized"))
		return
	}

	claims, err := m.parseRefresh(payload.RefreshToken)
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}
	sub, _ := authn.ClaimString(claims, "sub")
	if sub != principal.ID.String() {
		m.respond.Unauthorized(w, r, errors.New("refresh token does not match user"))
		return
	}

	jti, err := uuid.Parse(mustClaim(claims, "jti"))
	if err != nil {
		m.respond.Unauthorized(w, r, err)
		return
	}

	if err := m.refresh.Revoke(r.Context(), jti); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	_ = m.blocklist.Ban(r.Context(), jti.String(), time.Unix(int64FromExp(claims), 0))
	if principal.AccessJTI != "" {
		_ = m.blocklist.Ban(r.Context(), principal.AccessJTI, time.Unix(principal.AccessExp, 0))
	}

	w.WriteHeader(http.StatusNoContent)
}

// logoutAllHandler godoc
//
//	@Summary		Logout all sessions
//	@Description	Revoke every refresh token and bump token_version so existing access tokens die
//	@Tags			authentication
//	@Produce		json
//	@Success		204
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/authentication/logout-all [post]
func (m *Module) logoutAllHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("unauthorized"))
		return
	}

	if err := m.refresh.RevokeAllForUser(r.Context(), principal.ID); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if _, err := m.users.IncrementTokenVersion(r.Context(), principal.ID); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if principal.AccessJTI != "" {
		_ = m.blocklist.Ban(r.Context(), principal.AccessJTI, time.Unix(principal.AccessExp, 0))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *Module) issuePair(ctx context.Context, u *User) (*TokenResponse, error) {
	refreshJTI := uuid.New()
	refreshExp := time.Now().Add(m.refreshExp)
	if err := m.refresh.Insert(ctx, &RefreshSession{
		UserID:    u.ID,
		JTI:       refreshJTI,
		ExpiresAt: refreshExp,
	}); err != nil {
		return nil, err
	}
	return m.issuePairWithRefreshJTI(ctx, u, refreshJTI, refreshExp)
}

func (m *Module) issuePairWithRefreshJTI(_ context.Context, u *User, refreshJTI uuid.UUID, refreshExp time.Time) (*TokenResponse, error) {
	now := time.Now()
	accessJTI := uuid.New()
	accessExp := now.Add(m.tokenExp)

	access, err := m.authn.GenerateToken(jwt.MapClaims{
		"sub": u.ID.String(),
		"jti": accessJTI.String(),
		"typ": authn.TypeAccess,
		"ver": u.TokenVersion,
		"exp": accessExp.Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"iss": m.tokenIss,
		"aud": m.tokenAud,
	})
	if err != nil {
		return nil, err
	}

	refresh, err := m.authn.GenerateToken(jwt.MapClaims{
		"sub": u.ID.String(),
		"jti": refreshJTI.String(),
		"typ": authn.TypeRefresh,
		"ver": u.TokenVersion,
		"exp": refreshExp.Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"iss": m.tokenIss,
		"aud": m.tokenAud,
	})
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.tokenExp.Seconds()),
	}, nil
}

func (m *Module) parseRefresh(raw string) (jwt.MapClaims, error) {
	token, err := m.authn.ValidateToken(raw)
	if err != nil {
		return nil, err
	}
	claims, ok := authn.MapClaims(token)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	typ, _ := authn.ClaimString(claims, "typ")
	if typ != authn.TypeRefresh {
		return nil, fmt.Errorf("not a refresh token")
	}
	if _, ok := authn.ClaimString(claims, "jti"); !ok {
		return nil, fmt.Errorf("missing jti")
	}
	return claims, nil
}

func mustClaim(claims jwt.MapClaims, key string) string {
	s, _ := authn.ClaimString(claims, key)
	return s
}

func int64FromExp(claims jwt.MapClaims) int64 {
	raw, ok := claims["exp"]
	if !ok {
		return time.Now().Add(time.Hour).Unix()
	}
	switch v := raw.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	default:
		return time.Now().Add(time.Hour).Unix()
	}
}
