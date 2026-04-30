package carriers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// USPS webhook signature: HMAC-SHA256 over the raw body, BASE64-encoded
// (UPS does hex; USPS does base64; we asked once if they could match
// the others, and they could not). Header is X-USPS-Signature-256.
type USPS struct{}

func (USPS) Code() string { return "USPS" }

func (USPS) VerifySignature(req *http.Request, body []byte, signingKey string) error {
	provided := req.Header.Get("X-USPS-Signature-256")
	if provided == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(provided)) {
		return ErrInvalidSignature
	}
	return nil
}

type uspsPayload struct {
	TrackingID    string `json:"trackingId"`
	StatusCode    string `json:"statusCode"`
	StatusTime    string `json:"statusTime"`
	NotificationID string `json:"notificationId"`
}

func (USPS) ParseWebhook(body []byte) (*Event, error) {
	var p uspsPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &Event{
		CarrierCode:    "USPS",
		TrackingNumber: p.TrackingID,
		EventType:      p.StatusCode,
		EventTime:      p.StatusTime,
		RawCarrierID:   p.NotificationID,
	}, nil
}
