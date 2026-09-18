package post

import (
	"net/http"

	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Module struct {
	posts      *Store
	respond    *httpx.Responder
	guard      *authz.Guard
	writeLimit func(http.Handler) http.Handler
}

func New(db *pgxpool.Pool, respond *httpx.Responder, guard *authz.Guard, refs *refid.Generator, writeLimit func(http.Handler) http.Handler) *Module {
	if writeLimit == nil {
		writeLimit = func(next http.Handler) http.Handler { return next }
	}

	return &Module{
		posts:      NewStore(db, refs),
		respond:    respond,
		guard:      guard,
		writeLimit: writeLimit,
	}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/posts", func(r chi.Router) {
		r.With(m.guard.AuthToken, m.writeLimit).Post("/", m.createPostHandler)
		r.Get("/", m.listPostsHandler)

		r.Route("/{postID}", func(r chi.Router) {
			r.Use(m.postsContextMiddleware)
			r.Get("/", m.getPostHandler)
			r.With(m.guard.AuthToken, m.writeLimit, m.guard.OwnershipOrRole("moderator", postOwnerID)).Patch("/", m.updatePostHandler)
			r.With(m.guard.AuthToken, m.writeLimit, m.guard.OwnershipOrRole("admin", postOwnerID)).Delete("/", m.deletePostHandler)
		})
	})
}
