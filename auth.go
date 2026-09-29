package swh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Defaults for the public archive's Keycloak realm.
const (
	DefaultTokenURL = "https://auth.softwareheritage.org/auth/realms/SoftwareHeritage/protocol/openid-connect/token"
	DefaultClientID = "swh-web"
)

// tokenSource exchanges a long-lived offline token for bearer tokens.
type tokenSource struct {
	httpClient   *http.Client
	tokenURL     string
	clientID     string
	offlineToken string

	mu      sync.Mutex
	token   string
	expires time.Time
}

// refreshLeeway renews slightly early, so a token does not expire in flight.
const refreshLeeway = 30 * time.Second

func (s *tokenSource) bearer(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Now().Before(s.expires.Add(-refreshLeeway)) {
		return s.token, nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {s.clientID},
		"refresh_token": {s.offlineToken},
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("swh: building token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := s.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("swh: exchanging offline token: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("swh: exchanging offline token: %s", response.Status)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("swh: decoding token response: %w", err)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("swh: token response carried no access token")
	}

	s.token = body.AccessToken
	s.expires = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	return s.token, nil
}

// forget drops the cached token, so the next request exchanges a fresh one.
func (s *tokenSource) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = ""
}

type authTransport struct {
	base   http.RoundTripper
	tokens *tokenSource
}

func (t *authTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.attempt(request)
	if err != nil || response.StatusCode != http.StatusUnauthorized {
		return response, err
	}

	// The archive rejected the token. It may have been revoked server side
	// before the cached expiry, so exchange once more before giving up.
	_ = response.Body.Close()
	t.tokens.forget()
	return t.attempt(request)
}

func (t *authTransport) attempt(request *http.Request) (*http.Response, error) {
	bearer, err := t.tokens.bearer(request.Context())
	if err != nil {
		return nil, err
	}
	authenticated := request.Clone(request.Context())
	authenticated.Header.Set("Authorization", "Bearer "+bearer)
	return t.base.RoundTrip(authenticated)
}
