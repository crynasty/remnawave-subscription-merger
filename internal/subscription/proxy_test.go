package subscription

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crynasty/remnawave-subscription-merger/internal/remnawave"
)

type loggingTestTransport func(*http.Request) (*http.Response, error)

func (f loggingTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestProxyRequestLogging(t *testing.T) {
	for _, tc := range []struct {
		name          string
		path          string
		accept        string
		primary       string
		upstreamError bool
		responseType  string
		wantMessage   string
		wantStatus    int
	}{
		{"merged", "/subscription", "", `[{"remarks":"primary"}]`, false, "json", "configuration merged", http.StatusOK},
		{"merge error", "/subscription", "", `invalid JSON`, false, "json", "failed to build merged config; returning primary config", http.StatusOK},
		{"upstream error", "/subscription", "", "", true, "json", "upstream request failed", http.StatusBadGateway},
		{"asset", "/assets/app.js", "", "asset body", false, "browser", "", http.StatusOK},
		{"browser", "/subscription", "text/html", "browser body", false, "browser", "browser request detected; skipping merge", http.StatusOK},
		{"asset upstream error", "/assets/app.js", "", "", true, "browser", "upstream request failed", http.StatusBadGateway},
		{"browser upstream error", "/subscription", "text/html", "", true, "browser", "upstream request failed", http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger, previousTransport := slog.Default(), http.DefaultTransport
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() {
				slog.SetDefault(previousLogger)
				http.DefaultTransport = previousTransport
			})
			panelRequests := 0
			transport := loggingTestTransport(func(r *http.Request) (*http.Response, error) {
				body := tc.primary
				if r.URL.Host == "panel.test" {
					panelRequests++
					if strings.HasPrefix(r.URL.Path, "/api/users/by-username/") {
						body = `{"response":{"shortUuid":"limited"}}`
					} else if r.URL.Path == "/api/sub/limited/json" {
						body = `[{"remarks":"limited"}]`
					} else {
						t.Fatalf("unexpected panel request: %s", r.URL)
					}
				} else {
					if r.URL.Path != "/internal"+tc.path {
						t.Fatalf("unexpected rewritten path: %s", r.URL.Path)
					}
					if tc.upstreamError {
						return nil, errors.New("upstream unavailable")
					}
				}
				return &http.Response{
					StatusCode:    http.StatusOK,
					Header:        http.Header{"Content-Disposition": {"attachment; filename=primary"}},
					Body:          io.NopCloser(strings.NewReader(body)),
					ContentLength: int64(len(body)),
					Request:       r,
				}, nil
			})
			http.DefaultTransport = transport
			client := remnawave.NewClient("http://panel.test", "test-token", map[string]string{})
			upstream, err := url.Parse("http://subpage.test/internal")
			if err != nil {
				t.Fatal(err)
			}
			proxy := NewSubscriptionProxy(client, upstream)
			proxy.Transport = transport
			req := httptest.NewRequest(http.MethodGet, "http://merger.test"+tc.path+"?private=value", nil)
			req.Header.Set("User-Agent", "Happ/4.6.0")
			req.Header.Set("Accept", tc.accept)
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, req)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			output := logs.String()
			if tc.wantMessage == "" {
				if output != "" {
					t.Fatalf("asset request produced logs: %s", output)
				}
			} else if !strings.Contains(output, `msg="`+tc.wantMessage+`"`) {
				t.Fatalf("missing message %q: %s", tc.wantMessage, output)
			}
			for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
				if line == "" {
					continue
				}
				for _, field := range []string{"method=GET", "path=" + tc.path, "response_type=" + tc.responseType} {
					if strings.Count(line, field) != 1 {
						t.Errorf("missing or duplicated field %q: %s", field, line)
					}
				}
				if strings.Contains(line, "request_id=") || strings.Contains(line, "/internal") || strings.Contains(line, "private=value") {
					t.Errorf("unexpected request ID, rewritten path or query in log: %s", line)
				}
			}
			if tc.responseType == "browser" || tc.upstreamError {
				if panelRequests != 0 {
					t.Errorf("passthrough or failed request contacted panel %d times", panelRequests)
				}
				if !tc.upstreamError && recorder.Body.String() != tc.primary {
					t.Errorf("passthrough body changed: %s", recorder.Body.String())
				}
				if strings.Contains(output, "configuration merged") {
					t.Errorf("passthrough or failed request reported merge: %s", output)
				}
			} else if tc.name == "merged" {
				if !strings.Contains(output, `msg="merge started"`) || !strings.Contains(output, `msg="response body replaced"`) {
					t.Errorf("missing merge debug logs: %s", output)
				}
				if !strings.Contains(recorder.Body.String(), "primary") || !strings.Contains(recorder.Body.String(), "limited") {
					t.Errorf("unexpected merged body: %s", recorder.Body.String())
				}
			} else if recorder.Body.String() != tc.primary {
				t.Errorf("merge failure did not preserve primary body: %s", recorder.Body.String())
			}
		})
	}
}
