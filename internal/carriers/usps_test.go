package carriers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUSPS_VerifySignature_Good(t *testing.T) {
	body := []byte(`{"trackingId":"9400"}`)
	mac := hmac.New(sha256.New, []byte("k"))
	mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/usps", nil)
	req.Header.Set("X-USPS-Signature-256", sig)

	if err := (USPS{}).VerifySignature(req, body, "k"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestUSPS_ParseWebhook(t *testing.T) {
	ev, err := (USPS{}).ParseWebhook([]byte(`{"trackingId":"9400","statusCode":"D","statusTime":"t","notificationId":"n"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.CarrierCode != "USPS" || ev.TrackingNumber != "9400" {
		t.Fatalf("unexpected %+v", ev)
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	for _, code := range []string{"UPS", "ups", "uPs", "fedex", "FEDEX", "fdx", "DHL", "USPS"} {
		if _, err := Lookup(code); err != nil {
			t.Errorf("expected to find %q, got %v", code, err)
		}
	}
}

func TestLookup_Unknown(t *testing.T) {
	if _, err := Lookup("aramex"); err == nil {
		t.Fatal("expected unsupported error")
	}
}
