package event

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogExamplesValidate(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	validator := NewFileValidator(root)
	entries, err := os.ReadDir(filepath.Join(root, "docs", "eventing", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(root, "docs", "eventing", "examples", entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		e, err := Decode(raw)
		if err != nil {
			t.Fatalf("%s: decode: %v", entry.Name(), err)
		}
		if err := validator.Validate(context.Background(), e); err != nil {
			t.Fatalf("%s: schema: %v", entry.Name(), err)
		}
	}
}

func TestBreakingCatalogChangeIsRejected(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "eventing", "examples", "post-created.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].(map[string]any)
	delete(data, "owner_user_id")
	broken, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Decode(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewFileValidator(root).Validate(context.Background(), e); err == nil {
		t.Fatal("expected breaking payload to fail schema validation")
	} else if !IsPermanent(err) {
		t.Fatalf("schema failure should be permanent, got %v", err)
	}
}

func TestTypeChangeIsRejected(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "eventing", "examples", "post-created.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	payload["data"].(map[string]any)["owner_user_id"] = 42
	broken, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Decode(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewFileValidator(root).Validate(context.Background(), e); err == nil {
		t.Fatal("expected type change to fail schema validation")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
