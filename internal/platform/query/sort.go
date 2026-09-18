package query

import (
	"fmt"
	"net/http"
	"strings"
)

type Sort struct {
	By    string
	Order string
}

func ParseSort(r *http.Request, defaultBy, defaultOrder string, allowed []string) (Sort, error) {
	s := Sort{
		By:    defaultBy,
		Order: strings.ToLower(defaultOrder),
	}

	whitelist := make(map[string]struct{}, len(allowed))
	for _, col := range allowed {
		whitelist[col] = struct{}{}
	}

	qs := r.URL.Query()

	if sortBy := qs.Get("sort_by"); sortBy != "" {
		if _, ok := whitelist[sortBy]; !ok {
			return s, fmt.Errorf("invalid sort_by")
		}
		s.By = sortBy
	}

	if order := qs.Get("order"); order != "" {
		order = strings.ToLower(order)
		if order != "asc" && order != "desc" {
			return s, fmt.Errorf("invalid order")
		}
		s.Order = order
	}

	if s.Order != "asc" && s.Order != "desc" {
		s.Order = "desc"
	}

	if _, ok := whitelist[s.By]; !ok {
		s.By = defaultBy
	}

	return s, nil
}

func (s Sort) Clause() string {
	dir := "DESC"
	if s.Order == "asc" {
		dir = "ASC"
	}
	return s.By + " " + dir
}
