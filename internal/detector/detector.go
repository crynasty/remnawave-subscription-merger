package detector

import (
	"net/http"
	"strings"
)

type ResponseType int

const (
	ResponseBrowser ResponseType = iota
	ResponseXrayJSON
	ResponseMihomo
	ResponseStash
	ResponseSingBox
	ResponseClash
	ResponseBase64
)

func DetectClient(r *http.Request) ResponseType {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := strings.ToLower(r.Header.Get("Accept"))
	path := strings.ToLower(r.URL.Path)

	switch {
	case isBrowser(accept, path):
		return ResponseBrowser
	case strings.Contains(ua, "happ"):
		return ResponseXrayJSON
	case strings.HasPrefix(ua, "stash"):
		return ResponseStash
	case isMihomo(ua):
		return ResponseMihomo
	case isSingBox(ua):
		return ResponseSingBox
	case strings.HasPrefix(ua, "clash"):
		return ResponseClash
	default:
		return ResponseBase64
	}
}

func (r ResponseType) String() string {
	switch r {
	case ResponseBrowser:
		return "browser"
	case ResponseXrayJSON:
		return "json"
	case ResponseMihomo:
		return "mihomo"
	case ResponseStash:
		return "stash"
	case ResponseSingBox:
		return "singbox"
	case ResponseClash:
		return "clash"
	case ResponseBase64:
		return "base64"
	default:
		return "unknown"
	}
}

func IsAsset(r *http.Request) bool {
	path := strings.ToLower(r.URL.Path)
	if strings.HasPrefix(path, "/assets/") {
		return true
	}

	return false
}

func isBrowser(accept, path string) bool {
	if strings.Contains(accept, "text/html") || strings.HasPrefix(path, "/assets/") {
		return true
	}

	return false
}

func isMihomo(ua string) bool {
	ua = strings.ToLower(ua)
	return containsAny(ua,
		"flclash",
		"flclashx",
		"flowvy",
		"clash-verse",
		"koala-clash",
		"clash-meta",
		"clashmeta",
		"murge",
		"clashx meta",
		"mihomo",
		"clash-nyanpasu",
		"prizrak-box",
		"rabbithole",
	)

}

func isSingBox(ua string) bool {
	ua = strings.ToLower(ua)
	return containsAny(ua,
		"sing-box",
		"singbox",
		"karing",
		"sfa",
		"sfi",
		"sfm",
		"sft",
	)
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
