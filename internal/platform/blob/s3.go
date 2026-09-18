package blob

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Store struct {
	client    *s3.Client
	presign   *s3.PresignClient
	bucket    string
	driver    string
	publicURL string
}

func NewS3(cfg config.Storage) (*S3Store, error) {
	endpoint := cfg.S3.Endpoint
	if cfg.S3.UseSSL {
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "https://" + endpoint
		}
	} else if endpoint != "" && !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3.AccessKey, cfg.S3.SecretKey, "")),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = cfg.S3.PathStyle
	})

	return &S3Store{
		client:    client,
		presign:   s3.NewPresignClient(client),
		bucket:    cfg.S3.Bucket,
		driver:    strings.ToLower(cfg.Driver),
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
	}, nil
}

func (s *S3Store) Driver() string { return s.driver }

func (s *S3Store) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err := s.client.PutObject(ctx, input)
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) (*Object, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	contentType := "application/octet-stream"
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}

	return &Object{
		Key:         key,
		Size:        size,
		ContentType: contentType,
		Body:        out.Body,
	}, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Store) URL(ctx context.Context, key string) (string, error) {
	if s.publicURL != "" && !strings.Contains(s.publicURL, "/api/v1/files") {
		return s.publicURL + "/" + key, nil
	}

	presigned, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}
	if presigned == nil || presigned.URL == "" {
		return "", fmt.Errorf("empty presigned url")
	}
	return presigned.URL, nil
}
