package blob

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPutGetDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocal(dir, "http://localhost/files")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	payload := []byte("hello blob")
	if err := store.Put(ctx, "a.txt", "text/plain", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatal(err)
	}

	obj, err := store.Get(ctx, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer obj.Body.Close()
	got, _ := io.ReadAll(obj.Body)
	if string(got) != "hello blob" {
		t.Fatalf("got %q", got)
	}

	url, err := store.URL(ctx, "a.txt")
	if err != nil || url != "http://localhost/files/a.txt" {
		t.Fatalf("url=%q err=%v", url, err)
	}

	if err := store.Delete(ctx, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected missing file, err=%v", err)
	}
}
