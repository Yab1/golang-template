package user

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Yab1/golang-template/internal/platform/query"
)

type ListQuery struct {
	query.Page
	Sort     query.Sort
	Search   string
	Role     string
	IsActive *bool
}

func NewListQuery() ListQuery {
	return ListQuery{}
}

func (q ListQuery) Parse(r *http.Request) (ListQuery, error) {
	page, err := query.ParsePage(r)
	if err != nil {
		return q, err
	}
	q.Page = page

	sort, err := query.ParseSort(r, "u.created_at", "desc", []string{
		"u.created_at", "u.updated_at", "u.email", "u.username", "r.name", "r.level",
		"created_at", "updated_at", "email", "username",
	})
	if err != nil {
		return q, err
	}
	// Normalize bare column names to table-qualified for JOIN queries.
	switch sort.By {
	case "created_at":
		sort.By = "u.created_at"
	case "updated_at":
		sort.By = "u.updated_at"
	case "email":
		sort.By = "u.email"
	case "username":
		sort.By = "u.username"
	}
	q.Sort = sort

	q.Search = strings.TrimSpace(r.URL.Query().Get("search"))
	q.Role = strings.TrimSpace(r.URL.Query().Get("role"))

	if raw := strings.TrimSpace(r.URL.Query().Get("is_active")); raw != "" {
		switch raw {
		case "true", "1":
			v := true
			q.IsActive = &v
		case "false", "0":
			v := false
			q.IsActive = &v
		default:
			return q, fmt.Errorf("is_active must be true or false")
		}
	}

	return q, nil
}
