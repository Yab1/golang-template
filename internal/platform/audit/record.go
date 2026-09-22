package audit

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/authz"
)

// Record writes an audit entry from the request context. Failures are logged only.
func Record(log Logger, sugar *zap.SugaredLogger, r *http.Request, action, resourceType, resourceID string, meta map[string]any) {
	if log == nil {
		return
	}
	var actor *uuid.UUID
	if p := authz.PrincipalFrom(r); p != nil {
		id := p.ID
		actor = &id
	}
	err := log.Log(r.Context(), Entry{
		ActorID:      actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestID:    middleware.GetReqID(r.Context()),
		IP:           r.RemoteAddr,
		Meta:         meta,
	})
	if err != nil && sugar != nil {
		sugar.Warnw("audit log failed", "action", action, "resource_type", resourceType, "resource_id", resourceID, "error", err)
	}
}
