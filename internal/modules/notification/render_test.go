package notification

import "testing"

func TestRenderText(t *testing.T) {
	t.Parallel()
	got, err := renderBody(FormatText, "Hello {{.name}}. {{.message}}", map[string]any{"name": "Ada", "message": "Lab is ready"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello Ada. Lab is ready" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderMissingKey(t *testing.T) {
	t.Parallel()
	got, err := renderBody(FormatText, "Hello {{.name}}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderHTMLEscapes(t *testing.T) {
	t.Parallel()
	got, err := renderBody(FormatHTML, "<p>{{.message}}</p>", map[string]any{"message": "<b>x</b>"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "<p>&lt;b&gt;x&lt;/b&gt;</p>" {
		t.Fatalf("got %q", got)
	}
}
