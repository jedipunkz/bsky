package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://bsky.social/xrpc"

type Client struct {
	accessJWT  string
	refreshJWT string
	DID        string
	Handle     string
	baseURL    string
	http       *http.Client
	onRefresh  func(accessJWT, refreshJWT string)
}

func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SetOnRefresh(fn func(accessJWT, refreshJWT string)) {
	c.onRefresh = fn
}

func (c *Client) SetSession(accessJWT, refreshJWT, did, handle string) {
	c.accessJWT = accessJWT
	c.refreshJWT = refreshJWT
	c.DID = did
	c.Handle = handle
}

func (c *Client) IsAuthenticated() bool {
	return c.accessJWT != ""
}

// get calls an XRPC query endpoint, decoding the JSON response into out.
func (c *Client) get(endpoint string, params url.Values, out any) error {
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	return c.do(http.MethodGet, endpoint, nil, out, true)
}

// post calls an XRPC procedure endpoint with a JSON body. out may be nil when
// the response is not needed.
func (c *Client) post(endpoint string, body, out any) error {
	return c.do(http.MethodPost, endpoint, body, out, true)
}

// do performs an authenticated request. On an expired access token it refreshes
// the session and retries once; a second rejection is reported as an error
// rather than retried again.
func (c *Client) do(method, endpoint string, body, out any, retry bool) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, payload) //nolint:noctx
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessJWT)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized || isExpiredToken(data) {
		if !retry {
			return fmt.Errorf("session expired, please re-login")
		}
		if err := c.RefreshSession(); err != nil {
			return fmt.Errorf("session expired, please re-login")
		}
		return c.do(method, endpoint, body, out, false)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", strings.TrimPrefix(endpoint, "/"), errorMessage(data))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// errorMessage extracts the XRPC error message, falling back to the raw body.
func errorMessage(data []byte) string {
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &e); err == nil && e.Error != "" {
		if e.Message != "" {
			return e.Error + ": " + e.Message
		}
		return e.Error
	}
	return strings.TrimSpace(string(data))
}

func isExpiredToken(data []byte) bool {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &e)
	return e.Error == "ExpiredToken"
}

type SessionResp struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DID        string `json:"did"`
	Handle     string `json:"handle"`
}

func (c *Client) CreateSession(identifier, password string) (*SessionResp, error) {
	body, err := json.Marshal(struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}{Identifier: identifier, Password: password})
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Post(c.baseURL+"/com.atproto.server.createSession", "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		return nil, err
	}
	s, err := decodeSession(resp)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	c.SetSession(s.AccessJwt, s.RefreshJwt, s.DID, s.Handle)
	return s, nil
}

func (c *Client) RefreshSession() error {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/com.atproto.server.refreshSession", nil) //nolint:noctx
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.refreshJWT)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	s, err := decodeSession(resp)
	if err != nil {
		return fmt.Errorf("refresh failed: %w", err)
	}
	c.accessJWT = s.AccessJwt
	c.refreshJWT = s.RefreshJwt
	if c.onRefresh != nil {
		c.onRefresh(s.AccessJwt, s.RefreshJwt)
	}
	return nil
}

func decodeSession(resp *http.Response) (*SessionResp, error) {
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", errorMessage(data))
	}
	var s SessionResp
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
