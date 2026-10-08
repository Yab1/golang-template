package blob

import (
	"strings"
	"testing"
	"time"
)

func TestUploadKeyUsesClinicMonth(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 30, 21, 30, 0, 0, time.UTC)
	key := UploadKey("charts", "patient", "7ea2199e-6094-4697-ba21-6f2ee5437390", "photo", ".PNG", at)
	prefix := "charts/patient/7ea2199e-6094-4697-ba21-6f2ee5437390/photo/2026/10/"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".png") {
		t.Fatalf("key = %s", key)
	}
}

func TestDocumentKey(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	key := DocumentKey("invoice", "7ea2199e-6094-4697-ba21-6f2ee5437390", "pdf", at)
	prefix := "documents/invoice/7ea2199e-6094-4697-ba21-6f2ee5437390/2026/10/"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".pdf") {
		t.Fatalf("key = %s", key)
	}
}

func TestCleanExtDropsUnsafeSuffix(t *testing.T) {
	t.Parallel()
	key := UploadKey("brand", "clinic", "7ea2199e-6094-4697-ba21-6f2ee5437390", "logo", ".png/../x", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if strings.Contains(key, "..") || strings.HasSuffix(key, ".png") {
		t.Fatalf("key = %s", key)
	}
}
