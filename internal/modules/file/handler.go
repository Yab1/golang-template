package file

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/blob"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/httpx"
)

type Module struct {
	store        blob.Store
	respond      *httpx.Responder
	guard        authGuard
	audit        audit.Logger
	log          *zap.SugaredLogger
	writeLimit   func(http.Handler) http.Handler
	maxBytes     int64
	allowedTypes map[string]struct{}
}

type authGuard interface {
	AuthToken(http.Handler) http.Handler
}

type UploadResponse struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Driver      string `json:"driver"`
}

func New(
	store blob.Store,
	respond *httpx.Responder,
	guard authGuard,
	writeLimit func(http.Handler) http.Handler,
	cfg config.Files,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
) *Module {
	if writeLimit == nil {
		writeLimit = func(next http.Handler) http.Handler { return next }
	}
	if auditLog == nil {
		auditLog = audit.NewNop()
	}
	allowed := make(map[string]struct{}, len(cfg.AllowedTypes))
	for _, t := range cfg.AllowedTypes {
		allowed[strings.ToLower(t)] = struct{}{}
	}
	return &Module{
		store:        store,
		respond:      respond,
		guard:        guard,
		audit:        auditLog,
		log:          log,
		writeLimit:   writeLimit,
		maxBytes:     cfg.MaxBytes,
		allowedTypes: allowed,
	}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/files", func(r chi.Router) {
		r.With(m.guard.AuthToken, m.writeLimit).Post("/", m.uploadHandler)
		r.Get("/{key}", m.getHandler)
		r.With(m.guard.AuthToken, m.writeLimit).Delete("/{key}", m.deleteHandler)
	})
}

// uploadHandler godoc
//
//	@Summary		Upload a file
//	@Description	Multipart form field "file". Backend is local, s3, or minio via STORAGE_DRIVER.
//	@Tags			files
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"File"
//	@Success		201		{object}	httpx.ObjectResponse{result=UploadResponse}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files [post]
func (m *Module) uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, m.maxBytes+1024)
	if err := r.ParseMultipartForm(m.maxBytes); err != nil {
		m.respond.BadRequest(w, r, fmt.Errorf("file too large or invalid multipart"))
		return
	}

	fh, header, err := r.FormFile("file")
	if err != nil {
		m.respond.BadRequest(w, r, fmt.Errorf("missing form field file"))
		return
	}
	defer func() { _ = fh.Close() }()

	if header.Size > m.maxBytes {
		m.respond.BadRequest(w, r, fmt.Errorf("file exceeds max size"))
		return
	}

	contentType := header.Header.Get("Content-Type")
	buf := make([]byte, 512)
	n, _ := fh.Read(buf)
	detected := http.DetectContentType(buf[:n])
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detected
	}
	body := io.MultiReader(strings.NewReader(string(buf[:n])), fh)

	if len(m.allowedTypes) > 0 {
		if _, ok := m.allowedTypes[strings.ToLower(contentType)]; !ok {
			m.respond.BadRequest(w, r, fmt.Errorf("content type %s not allowed", contentType))
			return
		}
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	key := uuid.NewString() + ext

	if err := m.store.Put(r.Context(), key, contentType, body, header.Size); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	url, err := m.store.URL(r.Context(), key)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionCreate, "file", key, map[string]any{
		"size":         header.Size,
		"content_type": contentType,
		"driver":       m.store.Driver(),
	})

	if err := httpx.JSONResponse(w, http.StatusCreated, UploadResponse{
		Key:         key,
		URL:         url,
		Size:        header.Size,
		ContentType: contentType,
		Driver:      m.store.Driver(),
	}); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// getFileHandler godoc
//
//	@Summary		Get a file
//	@Description	Streams from local disk, or redirects to a presigned S3/MinIO URL
//	@Tags			files
//	@Produce		octet-stream
//	@Param			key	path		string	true	"Object key"
//	@Success		200	{file}		binary
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/files/{key} [get]
func (m *Module) getHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if m.store.Driver() != "local" {
		url, err := m.store.URL(r.Context(), key)
		if err != nil {
			m.respond.NotFound(w, r, err)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	obj, err := m.store.Get(r.Context(), key)
	if err != nil {
		m.respond.NotFound(w, r, err)
		return
	}
	defer func() { _ = obj.Body.Close() }()

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.Size))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj.Body)
}

// deleteFileHandler godoc
//
//	@Summary	Delete a file
//	@Tags		files
//	@Param		key	path		string	true	"Object key"
//	@Success	204	{string}	string	"No Content"
//	@Failure	401	{object}	httpx.ErrorResponse
//	@Failure	500	{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/files/{key} [delete]
func (m *Module) deleteHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if err := m.store.Delete(r.Context(), key); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionDelete, "file", key, nil)

	w.WriteHeader(http.StatusNoContent)
}
