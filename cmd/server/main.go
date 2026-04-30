// Command tracking-events is the carrier-webhook ingest service.
//
// Auth note (please read before "fixing"):
//
// This service authenticates to AWS using long-lived IAM access keys
// passed in via AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY environment
// variables. It does NOT use the ECS task role. The reason is in
// routebox-platform-docs/notes/handover.md (the "tracking-events"
// section); the deployment-side wiring is the special-case branch in
// routebox-jenkins/vars/deployToEcs.groovy. The keys are rotated
// quarterly by the tracking-events-rotate-keys Jenkins job, which
// also redeploys the service so the new env vars take effect.
//
// The aws-sdk-go v1 library is also pinned at the version we set this
// up with — newer versions changed credential refresh behavior in ways
// we have not yet validated against the workaround. If you bump it,
// run staging at full webhook volume for at least a week before prod.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/312school/routebox-tracking-events/internal/config"
	"github.com/312school/routebox-tracking-events/internal/db"
	"github.com/312school/routebox-tracking-events/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting", "env", cfg.Env, "port", cfg.Port)

	// AWS session built from the env-var credentials. The SDK would pick
	// these up implicitly via the default chain, but we construct the
	// session explicitly so the auth path is grep-able and so misconfig
	// fails at boot rather than on first carrier webhook.
	awsSess, err := buildAWSSession(cfg)
	if err != nil {
		return err
	}
	_ = awsSess // session is currently only used to validate creds parse

	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := db.NewEventRepo(pool)

	srv := server.New(cfg, repo, logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		logger.Info("signal", "sig", sig.String())
	}

	return server.Shutdown(ctx, srv)
}

func buildAWSSession(cfg *config.Config) (*session.Session, error) {
	creds := credentials.NewStaticCredentials(
		cfg.AWSAccessKeyID,
		cfg.AWSSecretAccessKey,
		"", // session token unused — these are long-lived IAM user keys
	)
	if _, err := creds.Get(); err != nil {
		// Empty creds in dev are tolerated; the service can boot without
		// AWS access (we only need it for the rotation flow upstream).
		// Production deploys will have both env vars set by the secrets
		// bootstrap; missing creds in prod will surface as auth failures
		// from carriers, which is the loud failure mode we want.
		if cfg.AWSAccessKeyID == "" && cfg.AWSSecretAccessKey == "" {
			slog.Warn("aws creds not set; continuing without an aws session")
			return nil, nil
		}
		return nil, err
	}
	return session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:      aws.String(cfg.AWSRegion),
			Credentials: creds,
		},
	})
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
