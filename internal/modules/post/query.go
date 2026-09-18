package post

import (
	"fmt"
	"net/http"
	"strings"

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

	sort, err := query.ParseSort(r, "created_at", "desc", sortColumns)
	if err != nil {
		return q, err
	}
	q.Sort = sort

	qs := r.URL.Query()

	if search := qs.Get("search"); search != "" {
		q.Search = search
	}

	if tags := qs.Get("tags"); tags != "" {
		q.Tags = strings.Split(tags, ",")
	}

	if userID := qs.Get("user_id"); userID != "" {
		id, err := uuid.Parse(userID)
		if err != nil {
			return q, fmt.Errorf("invalid user_id")
		}
		q.UserID = &id
	}

	return q, nil
}
