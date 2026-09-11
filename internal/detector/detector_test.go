package detector

import (
	"net/http"
	"testing"
)

func TestDetectClient_BrowserByHTMLAccept(t *testing.T) {
	req := newRequest(t, "/", "text/html,application/xhtml+xml", "")

	if got := DetectClient(req); got != ResponseBrowser {
		t.Fatalf("DetectClient() = %s, want browser", got.String())
	}
}

func TestDetectClient_BrowserByAssetsPath(t *testing.T) {
	req := newRequest(t, "/assets/index.js", "*/*", "")

	if got := DetectClient(req); got != ResponseBrowser {
		t.Fatalf("DetectClient() = %s, want browser", got.String())
	}
}

func TestDetectClient_DoesNotTreatNestedAssetsPathAsBrowser(t *testing.T) {
	req := newRequest(t, "/foo/assets/index.js", "*/*", "")

	if got := DetectClient(req); got == ResponseBrowser {
		t.Fatalf("DetectClient() = browser, want non-browser")
	}
}

func newRequest(t *testing.T, path, accept, userAgent string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://example.test"+path, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", userAgent)
	return req
}
