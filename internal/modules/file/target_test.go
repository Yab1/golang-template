package file

import "testing"

func TestTargetAllowList(t *testing.T) {
	t.Parallel()
	m := &Module{}
	m.Register("practitioner", "photo", Target{})
	m.Register("practitioner", "signature", Target{})
	if _, ok := m.target("Practitioner", "Photo"); !ok {
		t.Fatal("photo")
	}
	if _, ok := m.target("practitioner", "id_card"); ok {
		t.Fatal("unknown field accepted")
	}
	if _, ok := m.target("invoice", "photo"); ok {
		t.Fatal("unknown resource accepted")
	}
}
