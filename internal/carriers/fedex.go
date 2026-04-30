package carriers

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // FedEx's webhook spec uses SHA-1; we don't get to choose
	"encoding/hex"
	"encoding/json"
	"net/http"
)

// FedEx webhook signature: HMAC-SHA1 over (timestamp + "|" + body),
// presented in X-FedEx-Signature with the timestamp echoed in
// X-FedEx-Timestamp.
//
// Yes, SHA-1. Their spec was old when we wrote this and it's still old.
// We've asked. They've not moved.
type FedEx struct{}

func (FedEx) Code() string { return "FDX" }

func (FedEx) VerifySignature(req *http.Request, body []byte, signingKey string) error {
	provided := req.Header.Get("X-FedEx-Signature")
	ts := req.Header.Get("X-FedEx-Timestamp")
	if provided == "" || ts == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(ts))
	mac.Write([]byte("|"))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(provided)) {
		return ErrInvalidSignature
	}
	return nil
}

type fedexPayload struct {
	TrackingNumber string `json:"trackingNumber"`
	EventCode      string `json:"eventCode"`
	EventTimeUTC   string `json:"eventTimeUtc"`
	EventID        string `json:"eventId"`
}

func (FedEx) ParseWebhook(body []byte) (*Event, error) {
	var p fedexPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &Event{
		CarrierCode:    "FDX",
		TrackingNumber: p.TrackingNumber,
		EventType:      p.EventCode,
		EventTime:      p.EventTimeUTC,
		RawCarrierID:   p.EventID,
	}, nil
}
