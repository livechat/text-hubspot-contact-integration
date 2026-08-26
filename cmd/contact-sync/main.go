package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/livechat/text-hubspot-contact-sync/internal/contactsync"
	"github.com/livechat/text-hubspot-contact-sync/internal/hubspot"
	"github.com/livechat/text-hubspot-contact-sync/internal/textapi"
	"github.com/livechat/text-hubspot-contact-sync/internal/webhook"
)

type config struct {
	port               string
	textBasicAuth      string
	webhookSecret      string
	hubSpotAccessToken string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config, err := loadConfig()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	service := contactsync.New(
		textapi.New(config.textBasicAuth),
		hubspot.New(config.hubSpotAccessToken),
	)

	mux := http.NewServeMux()
	mux.Handle("/webhooks/text", webhook.NewHandler(config.webhookSecret, service, logger))
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		writer.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:              ":" + config.port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("starting webhook server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve webhook requests", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdownSignals
	logger.Info("shutting down webhook server")

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("shut down webhook server", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (config, error) {
	settings := config{
		port:               envOrDefault("PORT", "8080"),
		textBasicAuth:      strings.TrimSpace(os.Getenv("TEXT_BASIC_AUTH")),
		webhookSecret:      strings.TrimSpace(os.Getenv("TEXT_WEBHOOK_SECRET")),
		hubSpotAccessToken: strings.TrimSpace(os.Getenv("HUBSPOT_ACCESS_TOKEN")),
	}

	for name, value := range map[string]string{
		"TEXT_BASIC_AUTH":      settings.textBasicAuth,
		"TEXT_WEBHOOK_SECRET":  settings.webhookSecret,
		"HUBSPOT_ACCESS_TOKEN": settings.hubSpotAccessToken,
	} {
		if value == "" {
			return config{}, fmt.Errorf("%s is required", name)
		}
	}

	decodedBasicAuth, err := base64.StdEncoding.DecodeString(settings.textBasicAuth)
	if err != nil || !strings.Contains(string(decodedBasicAuth), ":") {
		return config{}, fmt.Errorf("TEXT_BASIC_AUTH must be one Base64-encoded account_id:personal_access_token credential")
	}

	return settings, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}

	return fallback
}
