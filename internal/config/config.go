package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type config struct {
	remnawaveToken     string
	remnawaveURL       string
	remnawaveHeaders   map[string]string
	subpageInternalURL string
	limitedPrefix      string
	proxyPort          string
	logLevel           string
}

var conf config

func RemnawaveToken() string {
	return conf.remnawaveToken
}

func RemnawaveURL() string {
	return conf.remnawaveURL
}

func RemnawaveHeaders() map[string]string {
	return conf.remnawaveHeaders
}

func SubpageInternalURL() string {
	return conf.subpageInternalURL
}

func LimitedPrefix() string {
	return conf.limitedPrefix
}

func ProxyPort() string {
	return conf.proxyPort
}

func LogLevel() string {
	return conf.logLevel
}

func InitConfig() {
	if os.Getenv("DISABLE_ENV_FILE") != "true" {
		if err := godotenv.Load(); err != nil {
			slog.Error("failed to read .env", "error", err)
		}
	}
	conf.remnawaveToken = mustEnv("REMNAWAVE_TOKEN")
	conf.remnawaveURL = mustEnv("REMNAWAVE_PANEL_URL")
	conf.remnawaveHeaders = func() map[string]string {
		v := os.Getenv("REMNAWAVE_HEADERS")
		if v != "" {
			headers := make(map[string]string)
			pairs := strings.Split(v, ";")
			for _, pair := range pairs {
				parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key != "" && value != "" {
						headers[key] = value
					}
				}
			}
			if len(headers) > 0 {
				slog.Debug("loaded remnawave headers", "count", len(headers))
				return headers
			}
		}
		return map[string]string{}
	}()
	conf.subpageInternalURL = mustEnv("SUBSCRIPTION_PAGE_INTERNAL_URL")
	conf.limitedPrefix = mustEnv("LIMITED_PREFIX")
	conf.proxyPort = mustEnv("PROXY_PORT")
	conf.logLevel = os.Getenv("LOG_LEVEL")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("env value is not set", "key", key)
		panic(1)
	}
	return v
}
