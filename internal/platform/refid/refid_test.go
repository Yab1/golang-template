package refid

import (
	"strings"
	"testing"
)

func TestNextFormat(t *testing.T) {
	g := New("gtm")
	got, err := g.Next("usr")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "GTM-USR-") {
		t.Fatalf("got %s", got)
	}
	if !Valid(got) {
		t.Fatalf("invalid %s", got)
	}
}

func TestNormalize(t *testing.T) {
	if Normalize("hb", "APP") != "HBA" {
		t.Fatalf("got %s", Normalize("hb", "APP"))
	}
	if Normalize("golang-template", "APP") != "GOL" {
		t.Fatalf("got %s", Normalize("golang-template", "APP"))
	}
}
