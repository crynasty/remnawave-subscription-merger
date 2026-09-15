package subscription

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/crynasty/remnawave-subscription-merger/internal/detector"
	"github.com/crynasty/remnawave-subscription-merger/internal/remnawave"
)

type subscriptionProxy struct {
	client   *remnawave.Client
	upstream *url.URL
}

type RequestIDKey struct{}
type isAssetKey struct{}

var requestSeq atomic.Uint64

func NewSubscriptionProxy(client *remnawave.Client, upstream *url.URL) *httputil.ReverseProxy {
	sp := subscriptionProxy{
		client:   client,
		upstream: upstream,
	}

	return &httputil.ReverseProxy{
		Rewrite:        sp.Rewrite,
		ModifyResponse: sp.ModifyResponse,
		ErrorHandler:   sp.ErrorHandler,
	}
}

func (sp *subscriptionProxy) Rewrite(pr *httputil.ProxyRequest) {
	isAsset := detector.IsAsset(pr.In)
	ctx := context.WithValue(pr.Out.Context(), isAssetKey{}, isAsset)

	var requestID uint64
	if !isAsset {
		requestID = requestSeq.Add(1)
		ctx = context.WithValue(ctx, RequestIDKey{}, requestID)
	}
	pr.Out = pr.Out.WithContext(ctx)

	if !isAsset {
		slog.Debug(
			"proxy request received",
			"request_id", requestID,
			"method", pr.In.Method,
		)
	}

	pr.SetURL(sp.upstream)
	setProxyRequestHeaders(pr)

	// Request an uncompressed, unmodified response body
	pr.Out.Header.Set("Accept-Encoding", "identity")

	// Prevent conditional requests and 304 Not Modified responses
	pr.Out.Header.Del("If-Modified-Since")
	pr.Out.Header.Del("If-None-Match")
}

func (sp *subscriptionProxy) ModifyResponse(r *http.Response) error {
	if isAsset, _ := r.Request.Context().Value(isAssetKey{}).(bool); isAsset {
		return nil
	}

	requestID, _ := r.Request.Context().Value(RequestIDKey{}).(uint64)
	responseType := detector.DetectClient(r.Request)
	logger := slog.With(
		"request_id", requestID,
		"response_type", responseType.String(),
	)

	if r.StatusCode != http.StatusOK {
		logger.Debug(
			"upstream response is not OK; skipping merge",
			"upstream_status", r.StatusCode,
		)
		return nil
	}

	logger.Debug(
		"modify response started",
		"upstream_status", r.StatusCode,
	)

	if responseType == detector.ResponseBrowser {
		logger.Debug("browser request detected; skipping merge")
		return nil
	}

	upstreamBody := r.Body
	defer upstreamBody.Close()

	primaryConf, err := io.ReadAll(upstreamBody)
	if err != nil {
		logger.Error("failed to read primary response body; returning original response", "error", err)
		originalBody(primaryConf, r)
		return nil
	}
	defer r.Body.Close()

	logger.Debug("primary config body read", "bytes", len(primaryConf))

	// Extract the primary username from [attachment; filename=<username>]
	contentDisposition := r.Header.Get("Content-Disposition")
	identity := extractSubscriptionRequestIdentity(r.Request.Header)

	mergedConf, err := sp.buildMergedConfig(r.Request.Context(), responseType, primaryConf, contentDisposition, identity)
	if err != nil {
		logger.Error("failed to build merged config; returning primary config", "error", err)
		originalBody(primaryConf, r)
		return nil
	}

	replaceResponseBody(r, mergedConf, responseType)

	logger.Info("configuration merged")
	logger.Debug(
		"response body replaced",
		"primary_bytes", len(primaryConf),
		"merged_bytes", len(mergedConf),
		"bytes_delta", len(mergedConf)-len(primaryConf),
		"result_differs_from_primary", !bytes.Equal(mergedConf, primaryConf),
	)
	return nil
}

func (sp *subscriptionProxy) ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	requestID := r.Context().Value(RequestIDKey{}).(uint64)

	slog.Error("upstream request failed",
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)

	w.WriteHeader(http.StatusBadGateway)
}

func setProxyRequestHeaders(pr *httputil.ProxyRequest) {
	if clientIP, _, err := net.SplitHostPort(pr.In.RemoteAddr); err == nil {
		if prior := pr.In.Header.Get("X-Forwarded-For"); prior != "" {
			pr.Out.Header.Set("X-Forwarded-For", prior+", "+clientIP)
		} else {
			pr.Out.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)

	proto := pr.In.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if pr.In.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	pr.Out.Header.Set("X-Forwarded-Proto", proto)
}

func extractSubscriptionRequestIdentity(
	header http.Header,
) remnawave.SubscriptionRequestIdentity {
	return remnawave.SubscriptionRequestIdentity{
		HWID:        header.Get("X-Hwid"),
		DeviceOS:    header.Get("X-Device-Os"),
		OSVersion:   header.Get("X-Ver-Os"),
		DeviceModel: header.Get("X-Device-Model"),
		UserAgent:   header.Get("User-Agent"),
	}
}
