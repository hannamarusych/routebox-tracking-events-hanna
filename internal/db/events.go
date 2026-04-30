package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CarrierEvent is the row written to carrier_events_raw.
//
// Schema reference: routebox-db-migrations/flyway/sql/V018__add_carrier_events_raw.sql
// V047 adds a metadata jsonb column but is NOT applied in prod, so this
// service does not write or read it. The README mentions a unique
// constraint over (carrier, tracking_number, event_type, event_time);
// that index does not actually exist in any environment we've checked
// recently. The dedup below is a SELECT-then-INSERT, race-prone, but
// matches the "best-effort dedup" the README documents.
type CarrierEvent struct {
	CarrierCode string
	RawBody     string
	Signature   string // SHA-256 hex of the body, used as our content fingerprint
}

// EventRepo wraps the pool and serves the (small) set of queries this
// service does.
type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

// IsRecentDuplicate returns true if a row with the same carrier_code +
// signature exists within the dedup window. Race-prone — see header
// comment.
func (r *EventRepo) IsRecentDuplicate(
	ctx context.Context,
	carrierCode, signature string,
	dedupWindow time.Duration,
) (bool, error) {
	if signature == "" {
		// Without a signature we can't dedupe. Don't pretend to.
		return false, nil
	}

	const q = `
		SELECT 1
		FROM carrier_events_raw
		WHERE carrier_code = $1
		  AND signature    = $2
		  AND received_at >= NOW() - $3::interval
		LIMIT 1`

	var one int
	err := r.pool.QueryRow(ctx, q, carrierCode, signature, dedupWindow).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Insert writes a single carrier_events_raw row. id is assigned by the
// DB default (gen_random_uuid()).
func (r *EventRepo) Insert(ctx context.Context, ev CarrierEvent) error {
	const q = `
		INSERT INTO carrier_events_raw
			(carrier_code, raw_body, signature, processed)
		VALUES ($1, $2, $3, FALSE)`

	_, err := r.pool.Exec(ctx, q, ev.CarrierCode, ev.RawBody, ev.Signature)
	return err
}

// Ping is used by /readyz.
func (r *EventRepo) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
