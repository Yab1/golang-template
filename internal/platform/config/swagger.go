package config

import "github.com/Yab1/golang-template/internal/platform/env"

type Swagger struct {
	Enabled          bool
	Title            string
	Description      string
	Version          string
	TermsOfService   string
	ContactName      string
	ContactURL       string
	ContactEmail     string
	LicenseName      string
	LicenseURL       string
	ExternalDocsURL  string
	ExternalDocsDesc string
}

func loadSwagger() Swagger {
	return Swagger{
		Enabled:          env.GetBool("SWAGGER_ENABLED", true),
		Title:            env.GetString("SWAGGER_TITLE", "Golang Template API"),
		Description:      env.GetString("SWAGGER_DESCRIPTION", "REST API template: auth (access+refresh), RBAC, posts, files, mail, rate limiting, reference ids."),
		Version:          env.GetString("SWAGGER_VERSION", "0.0.1"),
		TermsOfService:   env.GetString("SWAGGER_TOS", "https://www.google.com/policies/terms/"),
		ContactName:      env.GetString("SWAGGER_CONTACT_NAME", "Yeabsera"),
		ContactURL:       env.GetString("SWAGGER_CONTACT_URL", "https://yeabsera-dev.vercel.app/"),
		ContactEmail:     env.GetString("SWAGGER_CONTACT_EMAIL", "yeabsera.dev@gmail.com"),
		LicenseName:      env.GetString("SWAGGER_LICENSE_NAME", "Apache 2.0"),
		LicenseURL:       env.GetString("SWAGGER_LICENSE_URL", "https://www.apache.org/licenses/LICENSE-2.0.html"),
		ExternalDocsURL:  env.GetString("SWAGGER_EXTERNAL_DOCS_URL", "https://yeabsera-dev.vercel.app/"),
		ExternalDocsDesc: env.GetString("SWAGGER_EXTERNAL_DOCS_DESC", "External documentation"),
	}
}
