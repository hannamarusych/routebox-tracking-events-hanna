package carriers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sign(key, body string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestUPS_VerifySignature_Good(t *testing.T) {
	body := []byte(`{"trackingNumber":"1Z","statusType":{"code":"D"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", strings.NewReader(string(body)))
	req.Header.Set("X-UPS-Signature", sign("k", string(body)))

	if err := (UPS{}).VerifySignature(req, body, "k"); err != nil {
		t.Fatalf("expected ok signature, got %v", err)
	}
}

func TestUPS_VerifySignature_Tampered(t *testing.T) {
	body := []byte(`{"trackingNumber":"1Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", strings.NewReader(string(body)))
	req.Header.Set("X-UPS-Signature", sign("k", "different-body"))

	if err := (UPS{}).VerifySignature(req, body, "k"); err == nil {
		t.Fatal("expected signature mismatch, got nil")
	}
}

func TestUPS_VerifySignature_MissingHeader(t *testing.T) {
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", nil)
	if err := (UPS{}).VerifySignature(req, body, "k"); err == nil {
		t.Fatal("expected error on missing header")
	}
}

func TestUPS_ParseWebhook_PicksFields(t *testing.T) {
	ev, err := (UPS{}).ParseWebhook([]byte(
		`{"trackingNumber":"1ZTEST","statusType":{"code":"D","description":"Delivered"},"statusDateTime":"2026-01-02T03:04:05Z","eventId":"e1"}`,
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.TrackingNumber != "1ZTEST" || ev.EventType != "D" || ev.EventTime == "" {
		t.Fatalf("unexpected event %+v", ev)
	}
}
