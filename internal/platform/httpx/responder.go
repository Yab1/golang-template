package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type Responder struct {
	Logger *zap.SugaredLogger
}

func NewResponder(logger *zap.SugaredLogger) *Responder {
	return &Responder{Logger: logger}
}

func reqID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

func (rs *Responder) InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Errorw("internal error", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	WriteJSONError(w, http.StatusInternalServerError, "the server encountered a problem", reqID(r))
}

func (rs *Responder) Forbidden(w http.ResponseWriter, r *http.Request) {
	rs.Logger.Warnw("forbidden", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path)
	WriteJSONError(w, http.StatusForbidden, "forbidden", reqID(r))
}

func (rs *Responder) BadRequest(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Warnw("bad request", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	WriteJSONError(w, http.StatusBadRequest, err.Error(), reqID(r))
}

func (rs *Responder) Conflict(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Errorw("conflict response", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	WriteJSONError(w, http.StatusConflict, err.Error(), reqID(r))
}

func (rs *Responder) NotFound(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Warnw("not found error", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	WriteJSONError(w, http.StatusNotFound, "not found", reqID(r))
}

func (rs *Responder) Unauthorized(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Warnw("unauthorized error", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	WriteJSONError(w, http.StatusUnauthorized, "unauthorized", reqID(r))
}

func (rs *Responder) UnauthorizedBasic(w http.ResponseWriter, r *http.Request, err error) {
	rs.Logger.Warnw("unauthorized basic error", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path, "error", err.Error())
	w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
	WriteJSONError(w, http.StatusUnauthorized, "unauthorized", reqID(r))
}

func (rs *Responder) RateLimitExceeded(w http.ResponseWriter, r *http.Request, retryAfter string) {
	rs.Logger.Warnw("rate limit exceeded", "request_id", reqID(r), "method", r.Method, "path", r.URL.Path)
	w.Header().Set("Retry-After", retryAfter)
	WriteJSONError(w, http.StatusTooManyRequests, "rate limit exceeded, retry after: "+retryAfter, reqID(r))
}
