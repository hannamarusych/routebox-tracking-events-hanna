// Package carriers implements per-carrier webhook parsing and signature
// verification.
//
// Each carrier is its own implementation of Carrier. We keep them as
// individual files because the per-carrier specs drift apart often
// enough that one big switch statement got unreadable.
package carriers

import (
	"errors"
	"net/http"
)

// Event is the parsed, normalized payload extracted from a webhook body.
// Today this is intentionally lightweight — downstream services do the
// heavy lifting against carrier_events_raw.
type Event struct {
	CarrierCode    string
	TrackingNumber string
	EventType      string
	EventTime      string // ISO-8601 string; we don't parse to time.Time here
	RawCarrierID   string // carrier-side event id, useful for deduping
}

// Carrier is the per-carrier interface. Implementations live in this
// package, one file each.
type Carrier interface {
	// Code returns the canonical short code (e.g. "UPS").
	Code() string

	// VerifySignature checks the request signature against the body.
	// Returns nil on success. On failure returns an error whose
	// content does not need to be safe to surface to the caller — the
	// handler logs but does not return it.
	VerifySignature(req *http.Request, body []byte, signingKey string) error

	// ParseWebhook extracts an Event from the body. May return a partial
	// Event (e.g. without TrackingNumber) for some carriers' minimum
	// payloads — that's fine, raw_body is the source of truth.
	ParseWebhook(body []byte) (*Event, error)
}

// ErrInvalidSignature is returned by VerifySignature on mismatch.
var ErrInvalidSignature = errors.New("invalid signature")

// ErrUnsupportedCarrier is returned by Lookup when no impl exists.
var ErrUnsupportedCarrier = errors.New("unsupported carrier")

// Lookup returns the Carrier impl for the given carrier code (case-
// insensitive on the route param).
func Lookup(code string) (Carrier, error) {
	switch normalize(code) {
	case "ups":
		return UPS{}, nil
	case "fedex", "fdx":
		return FedEx{}, nil
	case "dhl":
		return DHL{}, nil
	case "usps":
		return USPS{}, nil
	default:
		return nil, ErrUnsupportedCarrier
	}
}

func normalize(code string) string {
	out := make([]byte, 0, len(code))
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
