package blob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	root      string
	publicURL string
}

func NewLocal(root, publicURL string) (*Local, error) {
	if root == "" {
		root = "./tmp/uploads"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{
		root:      root,
		publicURL: strings.TrimRight(publicURL, "/"),
	}, nil
}

func (s *Local) Driver() string { return "local" }

func (s *Local) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, body)
	return err
}

func (s *Local) Get(_ context.Context, key string) (*Object, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	contentType := "application/octet-stream"
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n > 0 {
		contentType = http.DetectContentType(buf[:n])
		_, _ = f.Seek(0, io.SeekStart)
	}

	return &Object{
		Key:         key,
		Size:        info.Size(),
		ContentType: contentType,
		Body:        f,
	}, nil
}

func (s *Local) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *Local) URL(_ context.Context, key string) (string, error) {
	return s.publicURL + "/" + key, nil
}

func (s *Local) path(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid key")
	}
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}
