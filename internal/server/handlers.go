package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/312school/routebox-tracking-events/internal/auth"
	"github.com/312school/routebox-tracking-events/internal/carriers"
	"github.com/312school/routebox-tracking-events/internal/config"
	"github.com/312school/routebox-tracking-events/internal/db"
)

// Handlers is the bag of HTTP handlers. Built once, kept on the server.
type Handlers struct {
	cfg    *config.Config
	repo   *db.EventRepo
	logger *slog.Logger
}

func NewHandlers(cfg *config.Config, repo *db.EventRepo, logger *slog.Logger) *Handlers {
	return &Handlers{cfg: cfg, repo: repo, logger: logger}
}

// Healthz: liveness, no dependencies.
func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Readyz: ping the DB on every call. No caching — same suboptimal
// pattern shipments-api uses.
func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.repo.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "reason": "db_unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Webhook is the per-carrier webhook receiver.
func (h *Handlers) Webhook(w http.ResponseWriter, r *http.Request) {
	carrierParam := chi.URLParam(r, "carrier")

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.cfg.MaxPayloadBytes))
	if err != nil {
		// MaxBytesReader returns a *http.MaxBytesError on overflow.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error": "payload_too_large",
				"max":   h.cfg.MaxPayloadBytes,
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read_body"})
		return
	}

	carrier, err := auth.Verify(r, body, carrierParam, h.cfg.CarrierSigningKeys)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrCarrierUnknown):
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown_carrier"})
		case errors.Is(err, auth.ErrSigningKeyMissing):
			h.logger.Error("missing_signing_key", "carrier", carrierParam,
				"request_id", RequestIDFrom(r.Context()))
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_misconfigured"})
		case errors.Is(err, carriers.ErrInvalidSignature):
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_signature"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "verify_failed"})
		}
		return
	}

	// Compute a content fingerprint for the dedupe lookup. Stored in the
	// signature column on insert.
	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])

	dedupWindow := time.Duration(h.cfg.DedupWindowHours) * time.Hour
	dup, err := h.repo.IsRecentDuplicate(r.Context(), carrier.Code(), fingerprint, dedupWindow)
	if err != nil {
		h.logger.Error("dedup_lookup_failed",
			"carrier", carrier.Code(),
			"request_id", RequestIDFrom(r.Context()),
			"err", err.Error(),
		)
		// Don't fail the webhook on a dedupe lookup failure — proceed
		// to insert and let the downstream tolerate the dupe (it does).
	}
	if dup {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deduped": true})
		return
	}

	// We don't fail the request on a parse error: raw_body is the
	// source of truth and downstream re-parses anyway. Log, store, ack.
	if _, perr := carrier.ParseWebhook(body); perr != nil {
		h.logger.Warn("parse_failed_storing_anyway",
			"carrier", carrier.Code(),
			"request_id", RequestIDFrom(r.Context()),
			"err", perr.Error(),
		)
	}

	if err := h.repo.Insert(r.Context(), db.CarrierEvent{
		CarrierCode: carrier.Code(),
		RawBody:     string(body),
		Signature:   fingerprint,
	}); err != nil {
		h.logger.Error("insert_failed",
			"carrier", carrier.Code(),
			"request_id", RequestIDFrom(r.Context()),
			"err", err.Error(),
		)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db_insert"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
