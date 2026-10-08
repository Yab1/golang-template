package file

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestObjectKeyKeepsSlashes(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Route("/files", func(r chi.Router) {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(objectKey(r)))
		})
	})
	cases := []string{
		"950fb67e-f483-403e-99cc-46f8290a3515.png",
		"charts/patient/7ea2199e-6094-4697-ba21-6f2ee5437390/photo/2026/10/950fb67e-f483-403e-99cc-46f8290a3515.png",
		"documents/invoice/7ea2199e-6094-4697-ba21-6f2ee5437390/2026/10/950fb67e-f483-403e-99cc-46f8290a3515.pdf",
	}
	for _, key := range cases {
		req := httptest.NewRequest(http.MethodGet, "/files/"+key, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Body.String() != key {
			t.Fatalf("key %s got %s", key, rec.Body.String())
		}
	}
}
