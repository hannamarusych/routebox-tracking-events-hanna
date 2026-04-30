package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/312school/routebox-tracking-events/internal/config"
	"github.com/312school/routebox-tracking-events/internal/db"
)

// requireTestDB returns a pgx pool against a Postgres reachable at
// TEST_DATABASE_URL. The caller is responsible for having migrations
// applied (the repo at routebox-db-migrations supplies them; the
// docker-compose helper at routebox-shipments-api/docker-compose.yml
// also produces a compatible DB).
func requireTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := db.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	// Wipe carrier_events_raw between tests so dedup logic is stable.
	_, err = pool.Exec(context.Background(),
		`TRUNCATE carrier_events_raw RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool, pool.Close
}

func newTestServer(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	cfg := &config.Config{
		Env:              "test",
		Port:             0,
		DedupWindowHours: 24,
		MaxPayloadBytes:  16 * 1024,
		CarrierSigningKeys: map[string]string{
			"ups": "test-ups-key",
		},
	}
	repo := db.NewEventRepo(pool)
	srv := New(cfg, repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv.Handler
}

func upsBody() []byte {
	return []byte(`{"trackingNumber":"1ZTEST123","statusType":{"code":"D"}}`)
}

func upsSign(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestWebhook_AcceptsGoodSignature(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	body := upsBody()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UPS-Signature", upsSign(body, "test-ups-key"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM carrier_events_raw WHERE carrier_code = 'UPS'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row, got %d", n)
	}
}

func TestWebhook_RejectsBadSignature(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	body := upsBody()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", bytes.NewReader(body))
	req.Header.Set("X-UPS-Signature", upsSign(body, "wrong-key"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWebhook_DedupesSecondCallToSameContent(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	body := upsBody()
	sig := upsSign(body, "test-ups-key")

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", bytes.NewReader(body))
		req.Header.Set("X-UPS-Signature", sig)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("iter %d status=%d", i, w.Code)
		}
	}

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM carrier_events_raw`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row after dedupe, got %d", n)
	}
}

func TestWebhook_RejectsOversizedPayload(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	huge := strings.Repeat("x", 32*1024) // > MaxPayloadBytes (16KB)
	body := []byte(huge)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/ups", bytes.NewReader(body))
	req.Header.Set("X-UPS-Signature", upsSign(body, "test-ups-key"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

func TestWebhook_UnknownCarrier(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/aramex", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	pool, cleanup := requireTestDB(t)
	defer cleanup()
	h := newTestServer(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// Help guard against sloppy refactors that move the migrations dir.
var _ = filepath.Join
