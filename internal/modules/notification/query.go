package notification

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/query"
)

func parseDeliveryQuery(r *http.Request, inboxUser *uuid.UUID) (deliveryFilter, error) {
	page, err := query.ParsePage(r)
	if err != nil {
		return deliveryFilter{}, err
	}
	f := deliveryFilter{Limit: page.Limit, Offset: page.Offset, UserID: inboxUser}
	if inboxUser != nil {
		f.Channel = ChannelInApp
		f.Status = StatusSent
		f.Unread = strings.EqualFold(r.URL.Query().Get("unread"), "true")
		return f, nil
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return deliveryFilter{}, fmt.Errorf("user_id must be a uuid")
		}
		f.UserID = &id
	}
	f.Channel = strings.TrimSpace(r.URL.Query().Get("channel"))
	if f.Channel != "" && !knownChannel(f.Channel) {
		return deliveryFilter{}, fmt.Errorf("unknown channel %q", f.Channel)
	}
	f.Status = strings.TrimSpace(r.URL.Query().Get("status"))
	switch f.Status {
	case "", StatusPending, StatusSending, StatusSent, StatusFailed, StatusSkipped:
	default:
		return deliveryFilter{}, fmt.Errorf("unknown status %q", f.Status)
	}
	return f, nil
}
