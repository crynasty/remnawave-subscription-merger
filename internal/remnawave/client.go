package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/crynasty/remnawave-subscription-merger/internal/config"
	"github.com/crynasty/remnawave-subscription-merger/internal/detector"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

type Response struct {
	Response struct {
		ShortUUID string `json:"shortUuid"`
	} `json:"response"`
}

type SubscriptionRequestIdentity struct {
	HWID        string
	DeviceOS    string
	OSVersion   string
	DeviceModel string
	UserAgent   string
}

func NewClient(baseURL, token string, headers map[string]string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	headers["Authorization"] = "Bearer " + token
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: headers,
			},
		},
		baseURL: baseURL,
	}
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())

	for key, value := range t.headers {
		r.Header.Set(key, value)
	}

	return t.base.RoundTrip(r)
}

// FetchLimitedShortUUID fetches the short UUID of the limited user
// associated with the given primary username.
func (c *Client) FetchLimitedShortUUID(ctx context.Context, primaryUsername string) (string, error) {
	limitedUsername := fmt.Sprintf("%s_%s", config.LimitedPrefix(), primaryUsername)
	reqURL := fmt.Sprintf("%s/api/users/by-username/%s", c.baseURL, limitedUsername)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request from url: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch limited user: unexpected HTTP status %d", resp.StatusCode)
	}

	var limitedUserResp Response
	err = json.NewDecoder(resp.Body).Decode(&limitedUserResp)
	if err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if limitedUserResp.Response.ShortUUID == "" {
		return "", errors.New("decode limited user: shortUUID is empty")
	}

	return limitedUserResp.Response.ShortUUID, nil
}

// FetchLimitedUserConfig fetches the limited user's subscription config
// by short UUID. The response type is specified when required.
func (c *Client) FetchLimitedUserConfig(ctx context.Context, shortUUID string, responseType detector.ResponseType, identity SubscriptionRequestIdentity) ([]byte, error) {
	var reqURL string

	switch responseType {
	case detector.ResponseBase64:
		reqURL = fmt.Sprintf("%s/api/sub/%s", c.baseURL, shortUUID)
	default:
		reqURL = fmt.Sprintf("%s/api/sub/%s/%s", c.baseURL, shortUUID, responseType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create new request: %w", err)
	}

	req.Header.Set("Accept-Encoding", "identity")
	setIdentityHeaders(req.Header, identity)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch limited config: unexpected HTTP status %d", resp.StatusCode)
	}

	limitedConf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read limited config response: %w", err)
	}

	return limitedConf, nil
}

func setIdentityHeaders(h http.Header, identity SubscriptionRequestIdentity) {
	if identity.HWID != "" {
		h.Set("X-Hwid", identity.HWID)
	}

	if identity.DeviceOS != "" {
		h.Set("X-Device-Os", identity.DeviceOS)
	}

	if identity.OSVersion != "" {
		h.Set("X-Ver-Os", identity.OSVersion)
	}
	if identity.DeviceModel != "" {
		h.Set("X-Device-Model", identity.DeviceModel)
	}

	if identity.UserAgent != "" {
		h.Set("User-Agent", identity.UserAgent)
	}
}
