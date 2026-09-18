package authz

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Yab1/golang-template/internal/platform/authn"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/google/uuid"
)

type contextKey string

const principalKey contextKey = "principal"

type Principal struct {
	ID        uuid.UUID
	RoleName  string
	RoleLevel int
	TokenVer  int
	AccessJTI string
	AccessExp int64
}

type UserFetcher interface {
	ByID(ctx context.Context, id uuid.UUID) (*Principal, error)
}

type RoleLevels interface {
	LevelOf(ctx context.Context, role string) (int, error)
}

type Guard struct {
	Authn       authn.Authenticator
	Users       UserFetcher
	Roles       RoleLevels
	Respond     *httpx.Responder
	Required    bool
	RBACEnabled bool
	DevToken    string
	DevUserID   uuid.UUID
	AllowDev    bool
	Blocklist   *authn.Blocklist
}

func NewGuard(authenticator authn.Authenticator, users UserFetcher, roles RoleLevels, respond *httpx.Responder) *Guard {
	return &Guard{
		Authn:       authenticator,
		Users:       users,
		Roles:       roles,
		Respond:     respond,
		Required:    true,
		RBACEnabled: true,
	}
}

func (g *Guard) AuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)

		if g.AllowDev && g.DevToken != "" && token == g.DevToken {
			principal, err := g.devPrincipal(r.Context())
			if err != nil {
				g.Respond.Unauthorized(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
			return
		}

		if token == "" {
			if !g.Required {
				principal, err := g.devPrincipal(r.Context())
				if err != nil {
					g.Respond.Unauthorized(w, r, err)
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
				return
			}
			g.Respond.Unauthorized(w, r, fmt.Errorf("authorization header is missing"))
			return
		}

		jwtToken, err := g.Authn.ValidateToken(token)
		if err != nil {
			g.Respond.Unauthorized(w, r, err)
			return
		}

		claims, ok := authn.MapClaims(jwtToken)
		if !ok {
			g.Respond.Unauthorized(w, r, fmt.Errorf("invalid token claims"))
			return
		}

		typ, _ := authn.ClaimString(claims, "typ")
		if typ != authn.TypeAccess {
			g.Respond.Unauthorized(w, r, fmt.Errorf("access token required"))
			return
		}

		sub, ok := authn.ClaimString(claims, "sub")
		if !ok {
			g.Respond.Unauthorized(w, r, fmt.Errorf("invalid subject claim"))
			return
		}

		jti, _ := authn.ClaimString(claims, "jti")
		blocked, err := g.Blocklist.Blocked(r.Context(), jti)
		if err != nil {
			g.Respond.Logger.Warnw("blocklist check failed", "error", err)
		} else if blocked {
			g.Respond.Unauthorized(w, r, fmt.Errorf("token revoked"))
			return
		}

		userID, err := uuid.Parse(sub)
		if err != nil {
			g.Respond.Unauthorized(w, r, err)
			return
		}

		principal, err := g.Users.ByID(r.Context(), userID)
		if err != nil {
			g.Respond.Unauthorized(w, r, err)
			return
		}

		if ver, ok := authn.ClaimInt(claims, "ver"); ok && ver != principal.TokenVer {
			g.Respond.Unauthorized(w, r, fmt.Errorf("token revoked"))
			return
		}

		principal.AccessJTI = jti
		if exp, ok := claims["exp"]; ok {
			switch v := exp.(type) {
			case float64:
				principal.AccessExp = int64(v)
			case int64:
				principal.AccessExp = v
			}
		}

		ctx := context.WithValue(r.Context(), principalKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Guard) OwnershipOrRole(requiredRole string, ownerID func(*http.Request) uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !g.RBACEnabled {
				next.ServeHTTP(w, r)
				return
			}

			principal := PrincipalFrom(r)
			if principal == nil {
				g.Respond.Unauthorized(w, r, fmt.Errorf("missing principal"))
				return
			}

			if ownerID(r) == principal.ID {
				next.ServeHTTP(w, r)
				return
			}

			level, err := g.Roles.LevelOf(r.Context(), requiredRole)
			if err != nil {
				g.Respond.InternalServerError(w, r, err)
				return
			}

			if principal.RoleLevel < level {
				g.Respond.Forbidden(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (g *Guard) devPrincipal(ctx context.Context) (*Principal, error) {
	if g.DevUserID != uuid.Nil {
		return g.Users.ByID(ctx, g.DevUserID)
	}
	if !g.Required {
		return nil, fmt.Errorf("auth disabled: set AUTH_DEV_USER_ID to a real user uuid")
	}
	return nil, fmt.Errorf("dev token configured but AUTH_DEV_USER_ID is empty")
}

func bearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func PrincipalFrom(r *http.Request) *Principal {
	principal, _ := r.Context().Value(principalKey).(*Principal)
	return principal
}
