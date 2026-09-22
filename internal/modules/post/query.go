package post

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/query"
)

var sortColumns = []string{"created_at", "updated_at", "title"}

type ListQuery struct {
	query.Page
	Sort   query.Sort
	Search string     `json:"search" validate:"omitempty,max=100"`
	Tags   []string   `json:"tags" validate:"omitempty,max=5,dive,max=100"`
	UserID *uuid.UUID `json:"user_id"`
}

func NewListQuery() ListQuery {
	return ListQuery{
		Page: query.NewPage(),
		Sort: query.Sort{By: "created_at", Order: "desc"},
		Tags: []string{},
	}
}

func (q ListQuery) Parse(r *http.Request) (ListQuery, error) {
	page, err := query.ParsePage(r)
	if err != nil {
		return q, err
	}
	q.Page = page

	sort, err := query.ParseSort(r, "created_a	t", "desc", sortColumns)
	if err != nil {
		return q, err
	}
	q.Sort = sort

	q.Search = query.OptionalString(r, "search")

	if tags := query.ParseCSV(r, "tags"); tags != nil {
		q.Tags = tags
	}

	userID, err := query.ParseUUID(r, "user_id")
	if err != nil {
		return q, err
	}
	q.UserID = userID

	// Cursor keyset is always (created_at, id). Reject conflicting sort_by.
	if q.UsingCursor() && q.Sort.By != "created_at" {
		return q, fmt.Errorf("cursor pagination requires sort_by=created_at")
	}

	return q, nil
}
