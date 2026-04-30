// Package auth orchestrates the carrier-agnostic side of webhook
// signature verification. The actual HMAC math lives in
// internal/carriers, one impl per carrier.
//
// CONTEXT FOR ANYONE TOUCHING THIS:
// This file is *fine*. The historical 1-in-50 webhook auth failure that
// made us migrate this service to long-lived AWS access keys was NOT in
// here — it was in the way our pinned aws-sdk-go v1 refreshed STS-derived
// credentials. The interaction caused something upstream to mangle the
// request before the bytes got to VerifySignature; the bytes that
// arrived were truthfully wrong, and so this code truthfully rejected
// them. That made debugging harder, not easier. See
// routebox-platform-docs/notes/handover.md "tracking-events" section.
//
// Don't try to fix this by adding leniency here. The leniency would
// silently double-process events when carriers retry, which is what
// broke billing reconciliation in the first place.
package auth

import (
	"errors"
	"net/http"

	"github.com/312school/routebox-tracking-events/internal/carriers"
)

// ErrCarrierUnknown is returned when the carrier path param doesn't map
// to a known impl.
var ErrCarrierUnknown = errors.New("unknown carrier")

// ErrSigningKeyMissing is returned when we have no key configured for
// the requested carrier. This is a configuration error and should fail
// loudly.
var ErrSigningKeyMissing = errors.New("no signing key configured for carrier")

// Verify dispatches to the per-carrier impl. signingKeys is the parsed
// JSON map from CARRIER_SIGNATURE_SECRETS, lowercased keys.
func Verify(
	req *http.Request,
	body []byte,
	carrierParam string,
	signingKeys map[string]string,
) (carriers.Carrier, error) {
	c, err := carriers.Lookup(carrierParam)
	if err != nil {
		return nil, ErrCarrierUnknown
	}
	key, ok := signingKeys[lower(c.Code())]
	if !ok || key == "" {
		// Fall back to the URL-param form if the operator keyed the JSON
		// by URL slug instead of canonical code (we've seen both shapes
		// in deploys).
		key, ok = signingKeys[lower(carrierParam)]
		if !ok || key == "" {
			return nil, ErrSigningKeyMissing
		}
	}
	if err := c.VerifySignature(req, body, key); err != nil {
		return nil, err
	}
	return c, nil
}

func lower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
