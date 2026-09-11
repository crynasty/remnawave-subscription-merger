package main

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/crynasty/remnawave-subscription-merger/internal/config"
	"github.com/crynasty/remnawave-subscription-merger/internal/remnawave"
	"github.com/crynasty/remnawave-subscription-merger/internal/subscription"
)

func main() {
	config.InitConfig()
	configureLogging(config.LogLevel())

	backend, err := url.Parse(config.SubpageInternalURL())
	if err != nil {
		slog.Error("invalid subscription page internal URL", "error", err)
		os.Exit(1)
	}

	client := remnawave.NewClient(config.RemnawaveURL(), config.RemnawaveToken(), config.RemnawaveHeaders())
	proxy := subscription.NewSubscriptionProxy(client, backend)

	mux := http.NewServeMux()
	mux.Handle("/", proxy)

	server := &http.Server{
		Addr:         ":" + config.ProxyPort(),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("merger started", "address", server.Addr)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listen and serve failed", "error", err)
		os.Exit(1)
	}
}

func configureLogging(value string) {
	level := slog.LevelInfo
	var parseError error

	if value != "" {
		parseError = level.UnmarshalText([]byte(value))
		if parseError != nil {
			level = slog.LevelInfo
		}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))

	if parseError != nil {
		slog.Warn(
			"invalid LOG_LEVEL; using INFO",
			"value", value,
		)
	}
}
