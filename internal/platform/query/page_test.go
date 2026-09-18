package query

import "testing"

func TestEncodeDecodeCursor(t *testing.T) {
	id := mustParseUUID(t, "11111111-1111-1111-1111-111111111111")
	ts := mustParseTime(t, "2026-09-18T12:00:00Z")

	enc := EncodeCursor(ts, id)
	if enc == "" {
		t.Fatal("empty cursor")
	}

	got, err := DecodeCursor(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id: got %s want %s", got.ID, id)
	}
	if !got.CreatedAt.Equal(ts) {
		t.Fatalf("time: got %s want %s", got.CreatedAt, ts)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	if _, err := DecodeCursor("not-a-cursor"); err == nil {
		t.Fatal("expected error")
	}
}
