// Package config reads runtime config from environment variables.
//
// AWS credentials are read by the AWS SDK from the standard env vars
// (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY) — see cmd/server/main.go
// for why this service does that instead of using the task role.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the parsed env. Treat as immutable after Load.
type Config struct {
	Env      string
	LogLevel string

	Port int

	DatabaseURL string

	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string

	// CarrierSigningKeys maps a carrier code (lowercased, e.g. "ups") to
	// its HMAC signing secret. Loaded from CARRIER_SIGNATURE_SECRETS as
	// a JSON map. The README still calls it CARRIER_WEBHOOK_SIGNING_KEYS
	// in places — both names are accepted for now.
	CarrierSigningKeys map[string]string

	DedupWindowHours int
	MaxPayloadBytes  int64
}

func mustGet(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("config: required env var " + k + " is empty")
	}
	return v
}

func getInt(k string, dflt int) int {
	v := os.Getenv(k)
	if v == "" {
		return dflt
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return dflt
	}
	return n
}

func getInt64(k string, dflt int64) int64 {
	v := os.Getenv(k)
	if v == "" {
		return dflt
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return dflt
	}
	return n
}

// Load reads config from the environment. Panics on missing required vars
// because there's no sensible default for them; we'd rather crash on boot
// than serve traffic with broken auth.
func Load() (*Config, error) {
	c := &Config{
		Env:      orDefault("ENV", "development"),
		LogLevel: strings.ToLower(orDefault("LOG_LEVEL", "info")),
		Port:     getInt("PORT", 8080),

		DatabaseURL: mustGet("DATABASE_URL"),

		AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSRegion:          orDefault("AWS_REGION", "us-east-1"),

		DedupWindowHours: getInt("DEDUP_WINDOW_HOURS", 24),
		MaxPayloadBytes:  getInt64("MAX_PAYLOAD_BYTES", 256*1024),
	}

	rawKeys := os.Getenv("CARRIER_SIGNATURE_SECRETS")
	if rawKeys == "" {
		// Older deploys used the verbose name; accept it too.
		rawKeys = os.Getenv("CARRIER_WEBHOOK_SIGNING_KEYS")
	}
	if rawKeys == "" {
		// In dev a missing map is allowed — handlers will reject all requests.
		c.CarrierSigningKeys = map[string]string{}
	} else {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(rawKeys), &parsed); err != nil {
			return nil, fmt.Errorf("parse CARRIER_SIGNATURE_SECRETS: %w", err)
		}
		// Lowercase the keys so the URL path lookup is case-insensitive.
		c.CarrierSigningKeys = make(map[string]string, len(parsed))
		for k, v := range parsed {
			c.CarrierSigningKeys[strings.ToLower(k)] = v
		}
	}

	return c, nil
}

func orDefault(k, dflt string) string {
	v := os.Getenv(k)
	if v == "" {
		return dflt
	}
	return v
}
