package blob

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Yab1/golang-template/internal/platform/config"
)

type Object struct {
	Key         string
	Size        int64
	ContentType string
	Body        io.ReadCloser
}

type Store interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (*Object, error)
	Delete(ctx context.Context, key string) error
	URL(ctx context.Context, key string) (string, error)
	Driver() string
}

func New(cfg config.Storage) (Store, error) {
	switch strings.ToLower(cfg.Driver) {
	case "", "local":
		return NewLocal(cfg.LocalDir, cfg.PublicURL)
	case "s3", "minio":
		return NewS3(cfg)
	default:
		return nil, fmt.Errorf("unknown storage driver %q (use local, s3, or minio)", cfg.Driver)
	}
}
