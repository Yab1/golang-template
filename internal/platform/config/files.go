package config

import (
	"strings"

	"github.com/Yab1/golang-template/internal/platform/env"
)

type Storage struct {
	Driver    string
	LocalDir  string
	PublicURL string
	S3        S3
}

type S3 struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	PathStyle bool
}

type Files struct {
	Enabled      bool
	MaxBytes     int64
	AllowedTypes []string
}

func loadStorage() Storage {
	driver := strings.ToLower(env.GetString("STORAGE_DRIVER", "local"))
	pathStyle := env.GetBool("S3_PATH_STYLE", driver == "minio")

	return Storage{
		Driver:    driver,
		LocalDir:  env.GetString("STORAGE_LOCAL_DIR", "./tmp/uploads"),
		PublicURL: env.GetString("STORAGE_PUBLIC_URL", "http://localhost:8080/api/v1/files"),
		S3: S3{
			Endpoint:  env.GetString("S3_ENDPOINT", "localhost:9000"),
			Region:    env.GetString("S3_REGION", "us-east-1"),
			Bucket:    env.GetString("S3_BUCKET", "uploads"),
			AccessKey: env.GetString("S3_ACCESS_KEY", "minioadmin"),
			SecretKey: env.GetString("S3_SECRET_KEY", "minioadmin"),
			UseSSL:    env.GetBool("S3_USE_SSL", false),
			PathStyle: pathStyle,
		},
	}
}

func loadFiles() Files {
	return Files{
		Enabled:      env.GetBool("FILES_ENABLED", true),
		MaxBytes:     int64(env.GetInt("FILE_MAX_BYTES", 10<<20)),
		AllowedTypes: env.GetCSV("FILE_ALLOWED_TYPES", []string{"image/jpeg", "image/png", "image/webp", "application/pdf"}),
	}
}
