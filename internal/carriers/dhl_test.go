package carriers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDHL_ParseV1XML(t *testing.T) {
	body := []byte(`<shipment-event><tracking-number>DHL1</tracking-number><event-code>OFD</event-code><event-time>2026-01-02T03:04:05Z</event-time><event-id>x1</event-id></shipment-event>`)
	ev, err := (DHL{}).ParseWebhook(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.TrackingNumber != "DHL1" || ev.EventType != "OFD" {
		t.Fatalf("unexpected %+v", ev)
	}
}

func TestDHL_VerifySignature_Good(t *testing.T) {
	body := []byte(`<shipment-event/>`)
	mac := hmac.New(sha256.New, []byte("k"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/dhl", nil)
	req.Header.Set("X-Dhl-Signature", sig)

	if err := (DHL{}).VerifySignature(req, body, "k"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}
