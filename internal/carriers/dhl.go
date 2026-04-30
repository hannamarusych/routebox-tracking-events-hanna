package carriers

// DHL v2 spec exists. We're still on v1. They've never enforced the
// migration. — see TODO in handover.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"net/http"
)

// DHL v1 webhook: HMAC-SHA256 over the raw body. Header is X-Dhl-Signature
// (lowercase 'hl'), and the body is XML, not JSON. We could pretty
// easily generalize the HMAC code with UPS's; we haven't because their
// next API release MAY change the algorithm.
type DHL struct{}

func (DHL) Code() string { return "DHL" }

func (DHL) VerifySignature(req *http.Request, body []byte, signingKey string) error {
	provided := req.Header.Get("X-Dhl-Signature")
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

// dhlV1Payload — minimal subset of the v1 schema. The v2 spec is rumored
// to be JSON.
type dhlV1Payload struct {
	XMLName        xml.Name `xml:"shipment-event"`
	TrackingNumber string   `xml:"tracking-number"`
	EventCode      string   `xml:"event-code"`
	EventTime      string   `xml:"event-time"`
	EventID        string   `xml:"event-id"`
}

func (DHL) ParseWebhook(body []byte) (*Event, error) {
	var p dhlV1Payload
	if err := xml.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &Event{
		CarrierCode:    "DHL",
		TrackingNumber: p.TrackingNumber,
		EventType:      p.EventCode,
		EventTime:      p.EventTime,
		RawCarrierID:   p.EventID,
	}, nil
}
