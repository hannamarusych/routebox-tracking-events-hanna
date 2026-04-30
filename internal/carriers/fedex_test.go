package carriers

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFedEx_VerifySignature_Good(t *testing.T) {
	body := []byte(`{"trackingNumber":"FX1"}`)
	ts := "20260102030405"
	mac := hmac.New(sha1.New, []byte("k"))
	mac.Write([]byte(ts))
	mac.Write([]byte("|"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/fedex", nil)
	req.Header.Set("X-FedEx-Signature", sig)
	req.Header.Set("X-FedEx-Timestamp", ts)

	if err := (FedEx{}).VerifySignature(req, body, "k"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestFedEx_VerifySignature_MissingTimestamp(t *testing.T) {
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/fedex", nil)
	req.Header.Set("X-FedEx-Signature", "deadbeef")
	if err := (FedEx{}).VerifySignature(req, body, "k"); err == nil {
		t.Fatal("expected error without timestamp")
	}
}
