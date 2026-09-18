package mailer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"

	"github.com/Yab1/golang-template/internal/platform/config"
)

type SES struct {
	client   *ses.Client
	from     string
	fromName string
}

func NewSES(cfg config.Mail) (*SES, error) {
	from, name, err := resolveFrom(cfg)
	if err != nil {
		return nil, err
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.SES.Region),
	}
	if cfg.SES.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.SES.AccessKey, cfg.SES.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, err
	}

	return &SES{
		client:   ses.NewFromConfig(awsCfg),
		from:     from,
		fromName: name,
	}, nil
}

func (s *SES) Driver() string {
	return "ses"
}

func (s *SES) Send(ctx context.Context, msg Message) error {
	from := s.from
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.from)
	}

	body := &types.Body{}
	if msg.Text != "" {
		body.Text = &types.Content{Data: aws.String(msg.Text), Charset: aws.String("UTF-8")}
	}
	if msg.HTML != "" {
		body.Html = &types.Content{Data: aws.String(msg.HTML), Charset: aws.String("UTF-8")}
	}

	_, err := s.client.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: []string{msg.To},
		},
		Message: &types.Message{
			Subject: &types.Content{Data: aws.String(msg.Subject), Charset: aws.String("UTF-8")},
			Body:    body,
		},
	})
	return err
}
