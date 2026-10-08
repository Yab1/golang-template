package file

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/blob"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/event"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/outbox"
	"github.com/Yab1/golang-template/internal/platform/stamp"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type Module struct {
	db           *pgxpool.Pool
	blobs        blob.Store
	files        *Store
	respond      *httpx.Responder
	guard        authGuard
	audit        audit.Logger
	log          *zap.SugaredLogger
	events       *outbox.Store
	eventTopic   string
	eventSource  string
	writeLimit   func(http.Handler) http.Handler
	maxBytes     int64
	allowedTypes map[string]struct{}
	targets      map[string]Target
}

type authGuard interface {
	AuthToken(http.Handler) http.Handler
}

type UploadResponse struct {
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Resource    string `json:"resource"`
	Field       string `json:"field"`
	ID          string `json:"id"`
}

func New(
	store blob.Store,
	db *pgxpool.Pool,
	respond *httpx.Responder,
	guard authGuard,
	writeLimit func(http.Handler) http.Handler,
	cfg config.Files,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
	events *outbox.Store,
	eventTopic string,
	eventSource string,
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
		db:           db,
		blobs:        store,
		files:        NewStore(db),
		respond:      respond,
		guard:        guard,
		audit:        auditLog,
		log:          log,
		events:       events,
		eventTopic:   eventTopic,
		eventSource:  eventSource,
		writeLimit:   writeLimit,
		maxBytes:     cfg.MaxBytes,
		allowedTypes: allowed,
	}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/files", func(r chi.Router) {
		r.With(m.guard.AuthToken, m.writeLimit).Post("/", m.uploadHandler)
		r.Get("/*", m.getHandler)
		r.With(m.guard.AuthToken, m.writeLimit).Delete("/*", m.deleteHandler)
	})
}

// uploadHandler godoc
//
//	@Summary		Upload a file onto a record
//	@Description	Multipart fields file, resource, field, and id. resource and field must be a registered pair. The server creates the storage key and attaches it in the same transaction. A replaced photo or logo deletes the previous object.
//	@Tags			files
//	@Accept			mpfd
//	@Produce		json
//	@Param			file		formData	file	true	"File"
//	@Param			resource	formData	string	true	"Resource"	Enums(upload)
//	@Param			field		formData	string	true	"Field"		Enums(file)
//	@Param			id			formData	string	true	"Record UUID or reference id"
//	@Success		201			{object}	httpx.ObjectResponse{result=UploadResponse}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Failure		409			{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/files [post]
func (m *Module) uploadHandler(w http.ResponseWriter, r *http.Request) {
	limit := m.maxBytes
	r.Body = http.MaxBytesReader(w, r.Body, limit+2048)
	if err := r.ParseMultipartForm(limit); err != nil {
		m.respond.BadRequest(w, r, fmt.Errorf("file too large or invalid multipart"))
		return
	}
	resource := strings.ToLower(strings.TrimSpace(r.FormValue("resource")))
	field := strings.ToLower(strings.TrimSpace(r.FormValue("field")))
	recordID := strings.TrimSpace(r.FormValue("id"))
	if resource == "" || field == "" || recordID == "" {
		m.respond.BadRequest(w, r, ErrSlot)
		return
	}
	target, ok := m.target(resource, field)
	if !ok || target.Bind == nil || target.Locate == nil || target.Class == "" {
		m.respond.BadRequest(w, r, ErrTarget)
		return
	}
	if target.MaxBytes > 0 && target.MaxBytes < limit {
		limit = target.MaxBytes
	}
	fh, header, err := r.FormFile("file")
	if err != nil {
		m.respond.BadRequest(w, r, fmt.Errorf("missing form field file"))
		return
	}
	defer func() { _ = fh.Close() }()
	buf, err := io.ReadAll(io.LimitReader(fh, limit+1))
	if err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if len(buf) == 0 || int64(len(buf)) > limit || (header.Size > 0 && header.Size > limit) {
		m.respond.BadRequest(w, r, fmt.Errorf("file exceeds max size"))
		return
	}
	contentType := header.Header.Get("Content-Type")
	detected := http.DetectContentType(buf)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detected
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if !m.typeOK(target, contentType) {
		m.respond.BadRequest(w, r, fmt.Errorf("content type %s not allowed", contentType))
		return
	}
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, fmt.Errorf("missing principal"))
		return
	}
	recordUUID, err := target.Locate(r.Context(), recordID)
	if err != nil {
		m.respondUpload(w, r, err)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	key := blob.UploadKey(target.Class, resource, recordUUID.String(), field, ext, time.Now())
	if err := m.blobs.Put(r.Context(), key, contentType, bytes.NewReader(buf), int64(len(buf))); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	actor := stamp.Ptr(principal.ID)
	object := &Object{
		Key: key, OwnerID: principal.ID, ContentType: contentType, Size: int64(len(buf)),
		Driver: m.blobs.Driver(), IsVisible: true, Metadata: []byte(`{}`), CreatedBy: actor, UpdatedBy: actor,
	}
	var replaceKey string
	err = storage.WithTx(m.db, r.Context(), func(tx pgx.Tx) error {
		if err := m.files.CreateTx(r.Context(), tx, object); err != nil {
			return err
		}
		replaced, err := target.Bind(r.Context(), r, recordID, object, buf, tx)
		if err != nil {
			return err
		}
		replaceKey = replaced
		if m.events == nil {
			return nil
		}
		e, err := event.New(event.NewParams{
			Source: m.eventSource, Type: event.TypeFileUploaded, Subject: "urn:file:" + key,
			DataSchema: event.SchemaFileUploaded, CorrelationID: middleware.GetReqID(r.Context()),
			TraceParent: r.Header.Get("traceparent"), AggregateVersion: 1,
			Data: map[string]any{
				"file_id": key, "size_bytes": len(buf), "media_type": contentType,
				"storage_driver": m.blobs.Driver(), "version": 1,
			},
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(r.Context(), tx, m.eventTopic, key, "file", e, fileEventHeaders(r))
	})
	if err != nil {
		_ = m.blobs.Delete(r.Context(), key)
		m.respondUpload(w, r, err)
		return
	}
	if replaceKey != "" && replaceKey != key {
		_ = storage.WithTx(m.db, r.Context(), func(tx pgx.Tx) error {
			old := &Object{Key: replaceKey}
			return m.files.SoftDeleteTx(r.Context(), tx, old, actor)
		})
	}
	audit.Record(m.audit, m.log, r, audit.ActionCreate, "file", key, map[string]any{
		"resource": resource, "field": field, "id": recordID, "size": len(buf), "content_type": contentType,
	})
	if err := httpx.JSONResponse(w, http.StatusCreated, UploadResponse{
		Key: key, Size: int64(len(buf)), ContentType: contentType, Resource: resource, Field: field, ID: recordID,
	}); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

func (m *Module) typeOK(target Target, contentType string) bool {
	allowed := m.allowedTypes
	if len(target.Types) > 0 {
		allowed = make(map[string]struct{}, len(target.Types))
		for _, kind := range target.Types {
			allowed[strings.ToLower(kind)] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[contentType]
	return ok
}

func (m *Module) respondUpload(w http.ResponseWriter, r *http.Request, err error) {
	var call *CallError
	switch {
	case errors.As(err, &call):
		switch call.Status {
		case http.StatusBadRequest:
			m.respond.BadRequest(w, r, call.Err)
		case http.StatusForbidden:
			m.respond.Forbidden(w, r, call.Err)
		case http.StatusNotFound:
			m.respond.NotFound(w, r, call.Err)
		case http.StatusConflict:
			m.respond.Conflict(w, r, call.Err)
		default:
			m.respond.InternalServerError(w, r, call.Err)
		}
	case errors.Is(err, ErrTarget), errors.Is(err, ErrSlot):
		m.respond.BadRequest(w, r, err)
	case errors.Is(err, authz.ErrForbidden):
		m.respond.Forbidden(w, r, err)
	case errors.Is(err, storage.ErrNotFound):
		m.respond.NotFound(w, r, err)
	case errors.Is(err, storage.ErrConflict), errors.Is(err, storage.ErrVersionMismatch):
		m.respond.Conflict(w, r, err)
	default:
		m.respond.InternalServerError(w, r, err)
	}
}

// getFileHandler godoc
//
//	@Summary		Get a file
//	@Description	Streams the stored bytes. The key may contain slashes. MinIO stays on the Docker network. The browser only talks to this API.
//	@Tags			files
//	@Produce		octet-stream
//	@Param			key	path		string	true	"Object key, including slashes"
//	@Success		200	{file}		binary
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/files/{key} [get]
func (m *Module) getHandler(w http.ResponseWriter, r *http.Request) {
	key := objectKey(r)
	if _, err := m.files.Get(r.Context(), key); err != nil {
		m.respond.NotFound(w, r, err)
		return
	}

	obj, err := m.blobs.Get(r.Context(), key)
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
//	@Param		key	path		string	true	"Object key, including slashes"
//	@Success	204	{string}	string	"No Content"
//	@Failure	401	{object}	httpx.ErrorResponse
//	@Failure	500	{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/files/{key} [delete]
func (m *Module) deleteHandler(w http.ResponseWriter, r *http.Request) {
	key := objectKey(r)
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, fmt.Errorf("missing principal"))
		return
	}
	object, err := m.files.Get(r.Context(), key)
	if err != nil {
		m.respond.NotFound(w, r, err)
		return
	}
	if object.OwnerID != principal.ID && principal.RoleName != "admin" {
		m.respond.Forbidden(w, r, nil)
		return
	}
	if err := storage.WithTx(m.db, r.Context(), func(tx pgx.Tx) error {
		if err := m.files.SoftDeleteTx(r.Context(), tx, object, stamp.Ptr(principal.ID)); err != nil {
			return err
		}
		if m.events == nil {
			return nil
		}
		e, err := event.New(event.NewParams{
			Source:           m.eventSource,
			Type:             event.TypeFileDeleted,
			Subject:          "urn:file:" + key,
			DataSchema:       event.SchemaFileDeleted,
			CorrelationID:    middleware.GetReqID(r.Context()),
			TraceParent:      r.Header.Get("traceparent"),
			AggregateVersion: object.Version,
			Data: map[string]any{
				"file_id": key,
				"version": object.Version,
			},
		})
		if err != nil {
			return err
		}
		return m.events.Enqueue(r.Context(), tx, m.eventTopic, key, "file", e, fileEventHeaders(r))
	}); err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}

	audit.Record(m.audit, m.log, r, audit.ActionDelete, "file", key, nil)

	w.WriteHeader(http.StatusNoContent)
}

func objectKey(r *http.Request) string {
	return strings.TrimPrefix(chi.URLParam(r, "*"), "/")
}

func fileEventHeaders(r *http.Request) map[string]string {
	headers := map[string]string{}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		headers["correlation-id"] = requestID
	}
	if traceParent := r.Header.Get("traceparent"); traceParent != "" {
		headers["traceparent"] = traceParent
	}
	return headers
}
