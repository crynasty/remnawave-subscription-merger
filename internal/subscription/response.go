package subscription

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/crynasty/remnawave-subscription-merger/internal/detector"
	"github.com/crynasty/remnawave-subscription-merger/internal/merge"
	"github.com/crynasty/remnawave-subscription-merger/internal/remnawave"
)

func (sp *subscriptionProxy) buildMergedConfig(
	ctx context.Context,
	responseType detector.ResponseType,
	primaryConf []byte,
	contentDisposition string,
	identity remnawave.SubscriptionRequestIdentity,
) ([]byte, error) {
	primaryUsername, err := extractUsername(contentDisposition)
	if err != nil {
		return nil, err
	}

	limitedShortUUID, err := sp.client.FetchLimitedShortUUID(ctx, primaryUsername)
	if err != nil {
		return nil, err
	}

	limitedConf, err := sp.client.FetchLimitedUserConfig(ctx, limitedShortUUID, responseType, identity)
	if err != nil {
		return nil, err
	}

	slog.Debug(
		"merge started",
		"request_id", ctx.Value(RequestIDKey{}),
		"response_type", responseType.String(),
		"primary_bytes", len(primaryConf),
		"limited_bytes", len(limitedConf),
	)

	mergedConf, err := mergeConfigs(primaryConf, limitedConf, responseType)
	if err != nil {
		return nil, fmt.Errorf("merge configs: %w", err)
	}

	return mergedConf, nil
}

func mergeConfigs(primary, limited []byte, responseType detector.ResponseType) ([]byte, error) {
	var mergedConf []byte
	var err error

	switch responseType {
	case detector.ResponseXrayJSON:
		mergedConf, err = merge.MergeXray(primary, limited)
	case detector.ResponseMihomo, detector.ResponseStash, detector.ResponseClash:
		mergedConf, err = merge.MergeClashLike(primary, limited)
	case detector.ResponseSingBox:
		mergedConf, err = merge.MergeSingBox(primary, limited)
	case detector.ResponseBase64:
		mergedConf, err = merge.MergeBase64(primary, limited)
	default:
		return nil, fmt.Errorf("unsupported response type: %q", responseType)
	}

	if err != nil {
		return nil, err
	}

	return mergedConf, nil
}

func originalBody(primaryConf []byte, r *http.Response) {
	r.Body = io.NopCloser(bytes.NewReader(primaryConf))
	r.ContentLength = int64(len(primaryConf))
	r.Header.Set("Content-Length", strconv.Itoa(len(primaryConf)))
}

func replaceResponseBody(r *http.Response, mergedBody []byte, responseType detector.ResponseType) {
	r.Body = io.NopCloser(bytes.NewReader(mergedBody))
	r.ContentLength = int64(len(mergedBody))
	r.Header.Set("Content-Length", strconv.Itoa(len(mergedBody)))

	if responseType == detector.ResponseXrayJSON {
		r.Header.Set("Content-Type", "application/json")
	}

	// The body no longer matches validators calculated by the upstream
	r.Header.Del("ETag")
}

func extractUsername(contentDisposition string) (string, error) {
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		err = fmt.Errorf("parse media type: %w", err)
		return "", err
	}

	primaryUsername, ok := params["filename"]
	if !ok {
		return "", errors.New("extract primary username: filename is missing from content-disposition")
	}

	return primaryUsername, nil
}
