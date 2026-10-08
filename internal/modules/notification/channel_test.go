package notification

import (
	"errors"
	"net/http"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	t.Parallel()
	if err := classifyStatus("sms", http.StatusNoContent); err != nil {
		t.Fatal(err)
	}
	if err := classifyStatus("sms", http.StatusTooManyRequests); err == nil || isPermanent(err) {
		t.Fatalf("429 should retry, got %v", err)
	}
	if err := classifyStatus("sms", http.StatusBadRequest); err == nil || !isPermanent(err) {
		t.Fatalf("400 should be permanent, got %v", err)
	}
	if err := classifyStatus("sms", http.StatusBadGateway); err == nil || isPermanent(err) {
		t.Fatalf("502 should retry, got %v", err)
	}
}

func TestPermanentUnwrap(t *testing.T) {
	t.Parallel()
	base := errors.New("chat not found")
	err := permanent(base)
	if !errors.Is(err, base) || !isPermanent(err) {
		t.Fatalf("permanent wrap failed: %v", err)
	}
}
