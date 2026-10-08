package notification

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/stamp"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

type SendPayload struct {
	UserID         uuid.UUID      `json:"user_id" validate:"required"`
	Category       string         `json:"category" validate:"required,oneof=welcome alert status transactional"`
	Channels       []string       `json:"channels" validate:"omitempty,dive,oneof=email sms push in_app telegram"`
	Data           map[string]any `json:"data"`
	IdempotencyKey string         `json:"idempotency_key" validate:"omitempty,max=128"`
}

type PreferenceItem struct {
	Channel     string `json:"channel" validate:"required,oneof=email sms push in_app telegram"`
	Category    string `json:"category" validate:"required,oneof=welcome alert status transactional"`
	Enabled     bool   `json:"enabled"`
	Destination string `json:"destination" validate:"omitempty,max=255"`
}

type PreferencePayload struct {
	Items []PreferenceItem `json:"items" validate:"required,min=1,dive"`
}

type TemplatePatch struct {
	Subject  *string `json:"subject" validate:"omitempty,max=200"`
	Body     *string `json:"body" validate:"omitempty"`
	Format   *string `json:"format" validate:"omitempty,oneof=text html markdown"`
	IsActive *bool   `json:"is_active"`
}

// sendHandler godoc
//
//	@Summary		Queue a notification
//	@Description	Writes one delivery per channel and returns. Disabled channels and opt-outs are stored as skipped. The poller sends pending rows.
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		SendPayload	true	"Notification"
//	@Success		200		{object}	httpx.ObjectResponse{result=[]Delivery}
//	@Success		201		{object}	httpx.ObjectResponse{result=[]Delivery}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/notifications [post]
func (m *Module) sendHandler(w http.ResponseWriter, r *http.Request) {
	var payload SendPayload
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
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	rows, replay, err := m.Enqueue(r.Context(), r, Request{
		UserID: payload.UserID, Category: payload.Category, Channels: payload.Channels,
		Data: payload.Data, IdempotencyKey: strings.TrimSpace(payload.IdempotencyKey),
		Actor: stamp.Ptr(principal.ID),
	})
	if err != nil {
		m.respondErr(w, r, err)
		return
	}
	audit.Record(m.audit, m.log, r, audit.ActionCreate, "notification", payload.UserID.String(), map[string]any{
		"category": payload.Category, "count": len(rows),
	})
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	if err := httpx.JSONResponse(w, status, rows); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// inboxHandler godoc
//
//	@Summary		List in-app notifications
//	@Description	Sent in-app rows for the current user. unread=true keeps rows that are not marked read.
//	@Tags			notifications
//	@Produce		json
//	@Param			unread	query		bool	false	"Unread only"
//	@Param			limit	query		int		false	"Page size"
//	@Param			offset	query		int		false	"Offset"
//	@Success		200		{object}	httpx.ListResponse{results=[]Delivery}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/notifications [get]
func (m *Module) inboxHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	f, err := parseDeliveryQuery(r, &principal.ID)
	if err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	page, err := m.store.ListDeliveries(r.Context(), f)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if page.Items == nil {
		page.Items = []*Delivery{}
	}
	if err := httpx.JSONList(w, http.StatusOK, page.Items, httpx.OffsetPagination(int64(page.Total), f.Limit, f.Offset)); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// readHandler godoc
//
//	@Summary	Mark an in-app notification read
//	@Tags		notifications
//	@Produce	json
//	@Param		notificationID	path		string	true	"Delivery UUID or reference id"
//	@Success	200				{object}	httpx.ObjectResponse{result=Delivery}
//	@Failure	401				{object}	httpx.ErrorResponse
//	@Failure	404				{object}	httpx.ErrorResponse
//	@Failure	500				{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/notifications/{notificationID}/read [post]
func (m *Module) readHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	row, err := m.store.GetDelivery(r.Context(), chi.URLParam(r, "notificationID"))
	if err != nil || row.UserID != principal.ID || row.Channel != ChannelInApp || row.Status != StatusSent {
		m.respond.NotFound(w, r, storage.ErrNotFound)
		return
	}
	if row.ReadAt == nil {
		updated, err := m.store.MarkRead(r.Context(), row.ID, principal.ID, stamp.Ptr(principal.ID))
		if err != nil {
			m.respondErr(w, r, err)
			return
		}
		row = updated
	}
	if err := httpx.JSONResponse(w, http.StatusOK, row); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// listDeliveriesHandler godoc
//
//	@Summary		List notification deliveries
//	@Description	Admin log of every channel attempt, including skipped and failed rows.
//	@Tags			notifications
//	@Produce		json
//	@Param			user_id	query		string	false	"User UUID"
//	@Param			channel	query		string	false	"Channel"
//	@Param			status	query		string	false	"Status"
//	@Param			limit	query		int		false	"Page size"
//	@Param			offset	query		int		false	"Offset"
//	@Success		200		{object}	httpx.ListResponse{results=[]Delivery}
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/notification-deliveries [get]
func (m *Module) listDeliveriesHandler(w http.ResponseWriter, r *http.Request) {
	f, err := parseDeliveryQuery(r, nil)
	if err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	page, err := m.store.ListDeliveries(r.Context(), f)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if page.Items == nil {
		page.Items = []*Delivery{}
	}
	if err := httpx.JSONList(w, http.StatusOK, page.Items, httpx.OffsetPagination(int64(page.Total), f.Limit, f.Offset)); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// getDeliveryHandler godoc
//
//	@Summary	Get a notification delivery
//	@Tags		notifications
//	@Produce	json
//	@Param		notificationID	path		string	true	"Delivery UUID or reference id"
//	@Success	200				{object}	httpx.ObjectResponse{result=Delivery}
//	@Failure	401				{object}	httpx.ErrorResponse
//	@Failure	403				{object}	httpx.ErrorResponse
//	@Failure	404				{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/notification-deliveries/{notificationID} [get]
func (m *Module) getDeliveryHandler(w http.ResponseWriter, r *http.Request) {
	row, err := m.store.GetDelivery(r.Context(), chi.URLParam(r, "notificationID"))
	if err != nil {
		m.respondErr(w, r, err)
		return
	}
	attempts, err := m.store.Attempts(r.Context(), row.ID)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	row.AttemptLog = attempts
	if err := httpx.JSONResponse(w, http.StatusOK, row); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// listPreferencesHandler godoc
//
//	@Summary		List notification preferences
//	@Description	Missing rows mean the channel is allowed. A row with enabled false opts out of that channel and category.
//	@Tags			notifications
//	@Produce		json
//	@Success		200	{object}	httpx.ListResponse{results=[]Preference}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/notification-preferences [get]
func (m *Module) listPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	rows, err := m.store.ListPreferences(r.Context(), principal.ID)
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if rows == nil {
		rows = []*Preference{}
	}
	limit := len(rows)
	if limit == 0 {
		limit = 1
	}
	if err := httpx.JSONList(w, http.StatusOK, rows, httpx.OffsetPagination(int64(len(rows)), limit, 0)); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// putPreferencesHandler godoc
//
//	@Summary		Set notification preferences
//	@Description	Upserts each item for the current user. Omitted pairs stay as they are.
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		PreferencePayload	true	"Preferences"
//	@Success		200		{object}	httpx.ListResponse{results=[]Preference}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Security		BearerAuth
//	@Router			/notification-preferences [put]
func (m *Module) putPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	var payload PreferencePayload
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	actor := stamp.Ptr(principal.ID)
	saved := make([]*Preference, 0, len(payload.Items))
	err := storage.WithTx(m.db, r.Context(), func(tx pgx.Tx) error {
		for _, item := range payload.Items {
			dest, err := cleanDestination(item.Channel, item.Destination)
			if err != nil {
				return err
			}
			row := &Preference{
				UserID: principal.ID, Channel: item.Channel, Category: item.Category,
				Enabled: item.Enabled, Destination: dest, IsVisible: true, CreatedBy: actor, UpdatedBy: actor,
			}
			if err := m.store.UpsertPreferenceTx(r.Context(), tx, row); err != nil {
				return err
			}
			saved = append(saved, row)
		}
		return nil
	})
	if err != nil {
		m.respondErr(w, r, err)
		return
	}
	audit.Record(m.audit, m.log, r, audit.ActionUpdate, "notification_preference", principal.ID.String(), map[string]any{
		"count": len(saved),
	})
	limit := len(saved)
	if limit == 0 {
		limit = 1
	}
	if err := httpx.JSONList(w, http.StatusOK, saved, httpx.OffsetPagination(int64(len(saved)), limit, 0)); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// listTemplatesHandler godoc
//
//	@Summary	List notification templates
//	@Tags		notifications
//	@Produce	json
//	@Success	200	{object}	httpx.ListResponse{results=[]Template}
//	@Failure	401	{object}	httpx.ErrorResponse
//	@Failure	403	{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/notification-templates [get]
func (m *Module) listTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := m.store.ListTemplates(r.Context())
	if err != nil {
		m.respond.InternalServerError(w, r, err)
		return
	}
	if rows == nil {
		rows = []*Template{}
	}
	limit := len(rows)
	if limit == 0 {
		limit = 1
	}
	if err := httpx.JSONList(w, http.StatusOK, rows, httpx.OffsetPagination(int64(len(rows)), limit, 0)); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

// patchTemplateHandler godoc
//
//	@Summary	Update a notification template
//	@Tags		notifications
//	@Accept		json
//	@Produce	json
//	@Param		templateID	path		string			true	"Template UUID or reference id"
//	@Param		payload		body		TemplatePatch	true	"Template fields"
//	@Success	200			{object}	httpx.ObjectResponse{result=Template}
//	@Failure	400			{object}	httpx.ErrorResponse
//	@Failure	401			{object}	httpx.ErrorResponse
//	@Failure	403			{object}	httpx.ErrorResponse
//	@Failure	404			{object}	httpx.ErrorResponse
//	@Security	BearerAuth
//	@Router		/notification-templates/{templateID} [patch]
func (m *Module) patchTemplateHandler(w http.ResponseWriter, r *http.Request) {
	principal := authz.PrincipalFrom(r)
	if principal == nil {
		m.respond.Unauthorized(w, r, errors.New("missing principal"))
		return
	}
	var payload TemplatePatch
	if err := httpx.ReadJSON(w, r, &payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	if err := httpx.Validate.Struct(payload); err != nil {
		m.respond.BadRequest(w, r, err)
		return
	}
	row, err := m.store.GetTemplate(r.Context(), chi.URLParam(r, "templateID"))
	if err != nil {
		m.respondErr(w, r, err)
		return
	}
	if payload.Subject != nil {
		row.Subject = strings.TrimSpace(*payload.Subject)
	}
	if payload.Body != nil {
		body := strings.TrimSpace(*payload.Body)
		if body == "" {
			m.respond.BadRequest(w, r, errors.New("body is required"))
			return
		}
		if _, err := renderBody(row.Format, body, map[string]any{}); err != nil {
			m.respond.BadRequest(w, r, ErrTemplate)
			return
		}
		row.Body = body
	}
	if payload.Format != nil {
		row.Format = *payload.Format
	}
	if payload.IsActive != nil {
		row.IsActive = *payload.IsActive
	}
	row.UpdatedBy = stamp.Ptr(principal.ID)
	if err := m.store.UpdateTemplate(r.Context(), row); err != nil {
		m.respondErr(w, r, err)
		return
	}
	audit.Record(m.audit, m.log, r, audit.ActionUpdate, "notification_template", row.ID.String(), map[string]any{
		"reference_id": row.ReferenceID, "version": row.Version,
	})
	if err := httpx.JSONResponse(w, http.StatusOK, row); err != nil {
		m.respond.InternalServerError(w, r, err)
	}
}

func (m *Module) respondErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		m.respond.NotFound(w, r, err)
	case errors.Is(err, ErrTemplate):
		m.respond.BadRequest(w, r, err)
	default:
		if strings.Contains(err.Error(), "destination") || strings.Contains(err.Error(), "unknown ") {
			m.respond.BadRequest(w, r, err)
			return
		}
		m.respond.InternalServerError(w, r, err)
	}
}
