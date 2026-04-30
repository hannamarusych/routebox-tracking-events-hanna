package carriers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

// UPS webhook signature: HMAC-SHA256 over the raw body, hex-encoded,
// presented in the X-UPS-Signature header.
type UPS struct{}

func (UPS) Code() string { return "UPS" }

func (UPS) VerifySignature(req *http.Request, body []byte, signingKey string) error {
	provided := req.Header.Get("X-UPS-Signature")
	if provided == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(provided)) {
		return ErrInvalidSignature
	}
	return nil
}

// upsPayload is what UPS sends. We accept extra fields without complaint.
type upsPayload struct {
	TrackingNumber string `json:"trackingNumber"`
	StatusType     struct {
		Code string `json:"code"`
		Desc string `json:"description"`
	} `json:"statusType"`
	StatusDateTime string `json:"statusDateTime"`
	EventID        string `json:"eventId"`
}

func (UPS) ParseWebhook(body []byte) (*Event, error) {
	var p upsPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &Event{
		CarrierCode:    "UPS",
		TrackingNumber: p.TrackingNumber,
		EventType:      p.StatusType.Code,
		EventTime:      p.StatusDateTime,
		RawCarrierID:   p.EventID,
	}, nil
}
