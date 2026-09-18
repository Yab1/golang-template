package user

import (
	"net/http"
	"time"

	"github.com/Yab1/golang-template/internal/platform/authn"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/mailer"
	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authGuard interface {
	AuthToken(http.Handler) http.Handler
}

type Module struct {
	users      *Store
	roles      *RoleStore
	refresh    *RefreshStore
	blocklist  *authn.Blocklist
	respond    *httpx.Responder
	authn      authn.Authenticator
	mail       mailer.Mailer
	guard      authGuard
	appName    string
	tokenExp   time.Duration
	refreshExp time.Duration
	tokenIss   string
	tokenAud   string
	authLimit  func(http.Handler) http.Handler
}

func New(db *pgxpool.Pool, respond *httpx.Responder, authenticator authn.Authenticator, token config.Token, mail mailer.Mailer, appName string, refs *refid.Generator, blocklist *authn.Blocklist, authLimit func(http.Handler) http.Handler) *Module {
	if authLimit == nil {
		authLimit = func(next http.Handler) http.Handler { return next }
	}
	if token.RefreshExp <= 0 {
		token.RefreshExp = 7 * 24 * time.Hour
	}
	if token.Exp <= 0 {
		token.Exp = 15 * time.Minute
	}

	return &Module{
		users:      NewStore(db, refs),
		roles:      NewRoleStore(db),
		refresh:    NewRefreshStore(db),
		blocklist:  blocklist,
		respond:    respond,
		authn:      authenticator,
		mail:       mail,
		appName:    appName,
		tokenExp:   token.Exp,
		refreshExp: token.RefreshExp,
		tokenIss:   token.Iss,
		tokenAud:   token.Aud,
		authLimit:  authLimit,
	}
}

func (m *Module) SetGuard(g authGuard) {
	m.guard = g
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/authentication", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(m.authLimit)
			r.Post("/token", m.createTokenHandler)
			r.Post("/refresh", m.refreshTokenHandler)
		})
		if m.guard != nil {
			r.Group(func(r chi.Router) {
				r.Use(m.guard.AuthToken)
				r.Post("/logout", m.logoutHandler)
				r.Post("/logout-all", m.logoutAllHandler)
			})
		}
	})

	r.Route("/users", func(r chi.Router) {
		r.Post("/", m.createUserHandler)
	})
}
