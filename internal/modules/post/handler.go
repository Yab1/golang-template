package post

import (
	"context"
	"errors"
	"net/http"

	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var errUnauthorized = errors.New("unauthorized")

type postKey string

const postCtx postKey = "post"

type CreatePostPayload struct {
	Title   string   `json:"title" validate:"required,max=255"`
	Content string   `json:"content" validate:"required,max=10000"`
	Tags    []string `json:"tags" validate:"omitempty,max=5,dive,max=100"`
}

type UpdatePostPayload struct {
	Title   *string  `json:"title" validate:"omitempty,max=255"`
	Content *string  `json:"content" validate:"omitempty,max=10000"`
	Tags    []string `json:"tags" validate:"omitempty,max=5,dive,max=100"`
}

// createPostHandler godoc
//
//	@Summary		Create a post
//	@Description	Create a post as the authenticated user
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		CreatePostPayload	true	"Post payload"
//	@Success		201		{object}	httpx.ObjectResponse{result=Post}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/posts [post]
func (m *Module) createPostHandler(w http.ResponseWriter, r *http.Request) {
	var payload CreatePostPayload
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

	p := &Post{
		UserID:  principal.ID,
		Title:   payload.Title,
		Content: payload.Content,
		Tags:    payload.Tags,
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	if err := m.posts.Create(r.Context(), p); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := httpx.JSONResponse(w, http.StatusCreated, p); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// listPostsHandler godoc
//
//	@Summary		List posts
//	@Description	List posts with pagination, sorting, search, and tag filters (public)
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int		false	"Page size (1-100)"	default(20)
//	@Param			offset	query		int		false	"Offset"			default(0)
//	@Param			sort_by	query		string	false	"Sort field"		Enums(created_at, updated_at, title)	default(created_at)
//	@Param			order	query		string	false	"Sort order"		Enums(asc, desc)					default(desc)
//	@Param			search	query		string	false	"Search title/content"
//	@Param			tags	query		string	false	"Comma-separated tags (AND / contains all)"
//	@Param			user_id	query		string	false	"Filter by author UUID"
//	@Success		200		{object}	httpx.ListResponse{results=[]Post}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/posts [get]
func (m *Module) listPostsHandler(w http.ResponseWriter, r *http.Request) {
	fq := NewListQuery()
	fq, err := fq.Parse(r)
	if err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	if err := httpx.Validate.Struct(fq); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	result, err := m.posts.List(r.Context(), fq)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	if err := httpx.JSONList(w, http.StatusOK, result.Items, httpx.Pagination{
		Total:  result.Total,
		Limit:  fq.Limit,
		Offset: fq.Offset,
	}); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// getPostHandler godoc
//
//	@Summary		Get a post
//	@Description	Get a post by ID (public)
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			postID	path		string	true	"Post UUID or reference_id (e.g. GTL-PST-A7K2M)"
//	@Success		200		{object}	httpx.ObjectResponse{result=Post}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/posts/{postID} [get]
func (m *Module) getPostHandler(w http.ResponseWriter, r *http.Request) {
	p := postFromCtx(r)

	if err := httpx.JSONResponse(w, http.StatusOK, p); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// updatePostHandler godoc
//
//	@Summary		Update a post
//	@Description	Owner or moderator+ can update. Optimistic locking via version.
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			postID	path		string				true	"Post UUID or reference_id (e.g. GTL-PST-A7K2M)"
//	@Param			payload	body		UpdatePostPayload	true	"Fields to update"
//	@Success		200		{object}	httpx.ObjectResponse{result=Post}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/posts/{postID} [patch]
func (m *Module) updatePostHandler(w http.ResponseWriter, r *http.Request) {
	p := postFromCtx(r)

	var payload UpdatePostPayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}

	if payload.Title != nil {
		p.Title = *payload.Title
	}
	if payload.Content != nil {
		p.Content = *payload.Content
	}
	if payload.Tags != nil {
		p.Tags = payload.Tags
	}

	if err := m.posts.Update(r.Context(), p); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			m.respond.NotFound(w, r, err)
		case errors.Is(err, storage.ErrVersionMismatch), errors.Is(err, storage.ErrConflict):
			m.respond.Conflict(w, r, err)
		default:
			m.respond.InternalServerError(w, r, err)
		}
		return
	}

	if err := httpx.JSONResponse(w, http.StatusOK, p); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// deletePostHandler godoc
//
//	@Summary		Delete a post
//	@Description	Owner or admin+ can delete
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			postID	path		string	true	"Post UUID or reference_id (e.g. GTL-PST-A7K2M)"
//	@Success		204		{string}	string	"No Content"
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/posts/{postID} [delete]
func (m *Module) deletePostHandler(w http.ResponseWriter, r *http.Request) {
	p := postFromCtx(r)

	if err := m.posts.Delete(r.Context(), p.ID); err != nil {
		switch {
		case errors.Is(err, storage.ErrNotFound):
			m.respond.NotFound(w, r, err)
		default:
			m.respond.InternalServerError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *Module) postsContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idParam := chi.URLParam(r, "postID")

		var (
			p   *Post
			err error
		)
		if id, parseErr := uuid.Parse(idParam); parseErr == nil {
			p, err = m.posts.GetByID(r.Context(), id)
		} else {
			p, err = m.posts.GetByReferenceID(r.Context(), idParam)
		}
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrNotFound):
				m.respond.NotFound(w, r, err)
			default:
				m.respond.InternalServerError(w, r, err)
			}
			return
		}

		ctx := context.WithValue(r.Context(), postCtx, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func postFromCtx(r *http.Request) *Post {
	p, _ := r.Context().Value(postCtx).(*Post)
	return p
}

func postOwnerID(r *http.Request) uuid.UUID {
	p := postFromCtx(r)
	if p == nil {
		return uuid.Nil
	}
	return p.UserID
}
