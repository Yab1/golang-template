package query

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParseCSV(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "tags=a,%20b,,"}}
	got := ParseCSV(r, "tags")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %#v", got)
	}
	if ParseCSV(r, "missing") != nil {
		t.Fatal("expected nil for missing")
	}
}

func TestParseUUID(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	r := &http.Request{URL: &url.URL{RawQuery: "user_id=" + id}}
	got, err := ParseUUID(r, "user_id")
	if err != nil || got == nil || got.String() != id {
		t.Fatalf("got %v err %v", got, err)
	}
	r2 := &http.Request{URL: &url.URL{RawQuery: ""}}
	got, err = ParseUUID(r2, "user_id")
	if err != nil || got != nil {
		t.Fatalf("expected nil,nil got %v %v", got, err)
	}
	r3 := &http.Request{URL: &url.URL{RawQuery: "user_id=nope"}}
	if _, err := ParseUUID(r3, "user_id"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseBool(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "active=true"}}
	v, set, err := ParseBool(r, "active")
	if err != nil || !set || !v {
		t.Fatalf("got %v %v %v", v, set, err)
	}
	r2 := &http.Request{URL: &url.URL{}}
	v, set, err = ParseBool(r2, "active")
	if err != nil || set || v {
		t.Fatalf("got %v %v %v", v, set, err)
	}
}
