package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

const DefaultAPIVersion = "1"

func init() {
	Validate = validator.New(validator.WithRequiredStructEnabled())
}

func WriteJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func ReadJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1_048_578 // 1mb
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(data)
}

func WriteJSONError(w http.ResponseWriter, status int, message, requestID string) error {
	if requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}

	return WriteJSON(w, status, &ErrorResponse{Error: message, RequestID: requestID})
}

// ErrorResponse is the shared API error envelope.
type ErrorResponse struct {
	Error     string `json:"error" example:"unauthorized"`
	RequestID string `json:"request_id,omitempty" example:"abc123"`
}

// Meta is top-level response metadata.
type Meta struct {
	Version    string      `json:"version" example:"1"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination is list-only metadata nested under meta.
type Pagination struct {
	Total  int64 `json:"total" example:"42"`
	Limit  int   `json:"limit" example:"20"`
	Offset int   `json:"offset" example:"0"`
}

// ObjectResponse is the success envelope for a single resource.
// Swagger: httpx.ObjectResponse{result=YourType}
type ObjectResponse struct {
	Status string `json:"status" example:"success" enums:"success"`
	Result any    `json:"result"`
	Meta   Meta   `json:"meta"`
}

// ListResponse is the success envelope for a collection.
// Swagger: httpx.ListResponse{results=[]YourType}
type ListResponse struct {
	Status  string `json:"status" example:"success" enums:"success"`
	Results any    `json:"results"`
	Meta    Meta   `json:"meta"`
}

// JSONResponse writes a single-object success envelope (result + meta.version).
func JSONResponse(w http.ResponseWriter, status int, result any) error {
	return JSONResponseVersion(w, status, result, DefaultAPIVersion)
}

// JSONResponseVersion is JSONResponse with an explicit API version.
func JSONResponseVersion(w http.ResponseWriter, status int, result any, version string) error {
	if version == "" {
		version = DefaultAPIVersion
	}
	return WriteJSON(w, status, &ObjectResponse{
		Status: "success",
		Result: result,
		Meta:   Meta{Version: version},
	})
}

// JSONList writes a list success envelope (results + meta.version + meta.pagination).
func JSONList(w http.ResponseWriter, status int, results any, page Pagination) error {
	return JSONListVersion(w, status, results, page, DefaultAPIVersion)
}

// JSONListVersion is JSONList with an explicit API version.
func JSONListVersion(w http.ResponseWriter, status int, results any, page Pagination, version string) error {
	if version == "" {
		version = DefaultAPIVersion
	}
	if results == nil {
		results = []any{}
	}
	return WriteJSON(w, status, &ListResponse{
		Status:  "success",
		Results: results,
		Meta: Meta{
			Version:    version,
			Pagination: &page,
		},
	})
}
