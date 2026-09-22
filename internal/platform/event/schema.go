package event

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
)

type SchemaValidator interface {
	Validate(context.Context, Event) error
}

type FileValidator struct {
	root  string
	mu    sync.RWMutex
	cache map[string]*gojsonschema.Schema
}

func NewFileValidator(root string) *FileValidator {
	return &FileValidator{root: root, cache: make(map[string]*gojsonschema.Schema)}
}

func (v *FileValidator) Validate(_ context.Context, e Event) error {
	schema, err := v.schema(e.DataSchema)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(catalogEventView(e))
	if err != nil {
		return err
	}
	return validateSchema(schema, payload)
}

type catalogEvent struct {
	SpecVersion      string          `json:"specversion"`
	ID               uuid.UUID       `json:"id"`
	Source           string          `json:"source"`
	Type             string          `json:"type"`
	Subject          string          `json:"subject"`
	Time             string          `json:"time"`
	DataContentType  string          `json:"datacontenttype"`
	DataSchema       string          `json:"dataschema"`
	CorrelationID    string          `json:"correlationid,omitempty"`
	CausationID      string          `json:"causationid,omitempty"`
	TraceParent      string          `json:"traceparent,omitempty"`
	AggregateVersion int64           `json:"aggregateversion"`
	Data             json.RawMessage `json:"data"`
}

func catalogEventView(e Event) catalogEvent {
	return catalogEvent{
		SpecVersion:      e.SpecVersion,
		ID:               e.ID,
		Source:           e.Source,
		Type:             e.Type,
		Subject:          e.Subject,
		Time:             e.Time.UTC().Format(time.RFC3339),
		DataContentType:  e.DataContentType,
		DataSchema:       e.DataSchema,
		CorrelationID:    e.CorrelationID,
		CausationID:      e.CausationID,
		TraceParent:      e.TraceParent,
		AggregateVersion: e.AggregateVersion,
		Data:             e.Data,
	}
}

func (v *FileValidator) schema(schemaURI string) (*gojsonschema.Schema, error) {
	v.mu.RLock()
	cached := v.cache[schemaURI]
	v.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	rel := strings.TrimPrefix(schemaURI, "/")
	path := filepath.Join(v.root, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Permanent(fmt.Errorf("read schema %s: %w", schemaURI, err))
	}
	envelope, err := filepath.Abs(filepath.Join(v.root, "docs", "eventing", "schemas", "cloudevents-envelope.v1.schema.json"))
	if err != nil {
		return nil, Permanent(err)
	}
	rewritten := strings.ReplaceAll(string(raw), `"./cloudevents-envelope.v1.schema.json"`, `"file://`+envelope+`"`)
	compiled, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(rewritten))
	if err != nil {
		return nil, Permanent(fmt.Errorf("compile schema %s: %w", schemaURI, err))
	}
	v.mu.Lock()
	v.cache[schemaURI] = compiled
	v.mu.Unlock()
	return compiled, nil
}

type RegistryValidator struct {
	baseURL string
	client  *http.Client
	mu      sync.RWMutex
	cache   map[string]*gojsonschema.Schema
}

func NewRegistryValidator(baseURL string, client *http.Client) *RegistryValidator {
	if client == nil {
		client = http.DefaultClient
	}
	return &RegistryValidator{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
		cache:   make(map[string]*gojsonschema.Schema),
	}
}

func (v *RegistryValidator) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/subjects", nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("ping schema registry: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping schema registry: status %d", resp.StatusCode)
	}
	return nil
}

func (v *RegistryValidator) Validate(ctx context.Context, e Event) error {
	schema, err := v.schema(ctx, e.DataSchema)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(catalogEventView(e))
	if err != nil {
		return err
	}
	return validateSchema(schema, payload)
}

func (v *RegistryValidator) schema(ctx context.Context, schemaURI string) (*gojsonschema.Schema, error) {
	v.mu.RLock()
	cached := v.cache[schemaURI]
	v.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	subject, version, err := parseSchemaURI(schemaURI)
	if err != nil {
		return nil, Permanent(err)
	}
	endpoint := fmt.Sprintf("%s/subjects/%s/versions/%s", v.baseURL, subject, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch schema: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("fetch schema: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var registryResponse struct {
		Schema     string `json:"schema"`
		SchemaType string `json:"schemaType"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registryResponse); err != nil {
		return nil, fmt.Errorf("decode schema response: %w", err)
	}
	if registryResponse.SchemaType != "" && !strings.EqualFold(registryResponse.SchemaType, "JSON") {
		return nil, Permanent(fmt.Errorf("schema %s is not JSON Schema", schemaURI))
	}
	compiled, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(registryResponse.Schema))
	if err != nil {
		return nil, Permanent(fmt.Errorf("compile schema %s: %w", schemaURI, err))
	}
	v.mu.Lock()
	v.cache[schemaURI] = compiled
	v.mu.Unlock()
	return compiled, nil
}

func parseSchemaURI(raw string) (subject, version string, err error) {
	if strings.HasPrefix(raw, "/docs/eventing/schemas/") {
		name := strings.TrimSuffix(strings.TrimPrefix(raw, "/docs/eventing/schemas/"), ".schema.json")
		return "dev.yab1.golangtemplate." + name, "latest", nil
	}
	const prefix = "registry://"
	if !strings.HasPrefix(raw, prefix) {
		return "", "", fmt.Errorf("invalid schema URI %q", raw)
	}
	parts := strings.Split(strings.TrimPrefix(raw, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid schema URI %q", raw)
	}
	return parts[0], parts[1], nil
}

func validateSchema(schema *gojsonschema.Schema, payload []byte) error {
	result, err := schema.Validate(gojsonschema.NewBytesLoader(payload))
	if err != nil {
		return fmt.Errorf("validate event: %w", err)
	}
	if result.Valid() {
		return nil
	}
	errs := make([]string, 0, len(result.Errors()))
	for _, validationErr := range result.Errors() {
		errs = append(errs, validationErr.String())
	}
	return Permanent(fmt.Errorf("event schema validation failed: %s", strings.Join(errs, "; ")))
}
