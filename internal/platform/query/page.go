package query

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Page struct {
	Limit  int `json:"limit" validate:"gte=1,lte=100"`
	Offset int `json:"offset" validate:"gte=0"`
}

func NewPage() Page {
	return Page{
		Limit:  DefaultLimit,
		Offset: 0,
	}
}

func ParsePage(r *http.Request) (Page, error) {
	page := NewPage()
	qs := r.URL.Query()

	if limit := qs.Get("limit"); limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return page, fmt.Errorf("invalid limit")
		}
		page.Limit = n
	}

	if offset := qs.Get("offset"); offset != "" {
		n, err := strconv.Atoi(offset)
		if err != nil {
			return page, fmt.Errorf("invalid offset")
		}
		page.Offset = n
	}

	return page, nil
}
