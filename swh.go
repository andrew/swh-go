// Package swh is a client for the Software Heritage Web API.
//
// The typed request methods in [Client.API] are generated from an OpenAPI
// description of the archive, which is itself generated from swh-web's own
// endpoint documentation. Everything else in this package covers behaviour
// the description does not express: authentication, rate limits, and the
// Link header the archive paginates with.
package swh

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andrew/swh-go/internal/gen"
)

// DefaultBaseURL is the public Software Heritage archive.
const DefaultBaseURL = "https://archive.softwareheritage.org"

// Client calls the Software Heritage Web API.
type Client struct {
	// API holds the generated request methods, one per documented endpoint.
	API *gen.ClientWithResponses

	httpClient *http.Client
	baseURL    string
}

// Option configures a [Client].
type Option func(*config)

type config struct {
	baseURL      string
	httpClient   *http.Client
	offlineToken string
	tokenURL     string
	clientID     string
}

// WithBaseURL calls a different deployment, such as a mirror or a staging
// instance.
func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

// WithHTTPClient supplies the underlying client, for callers that need their
// own timeouts, proxy or instrumentation.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) { c.httpClient = client }
}

// WithOfflineToken authenticates requests. The token is the long-lived one
// issued by the archive's account page, which is exchanged for short-lived
// bearer tokens as needed. Authenticated requests are rate limited far more
// generously than anonymous ones.
func WithOfflineToken(token string) Option {
	return func(c *config) { c.offlineToken = token }
}

// WithAuthEndpoint overrides where offline tokens are exchanged, for
// deployments running their own Keycloak realm.
func WithAuthEndpoint(tokenURL, clientID string) Option {
	return func(c *config) {
		c.tokenURL = tokenURL
		c.clientID = clientID
	}
}

// New builds a client. Without [WithOfflineToken] it makes anonymous requests.
func New(options ...Option) (*Client, error) {
	cfg := config{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
		tokenURL:   DefaultTokenURL,
		clientID:   DefaultClientID,
	}
	for _, option := range options {
		option(&cfg)
	}

	httpClient := *cfg.httpClient
	if cfg.offlineToken != "" {
		httpClient.Transport = &authTransport{
			base: transportOf(cfg.httpClient),
			tokens: &tokenSource{
				httpClient:   cfg.httpClient,
				tokenURL:     cfg.tokenURL,
				clientID:     cfg.clientID,
				offlineToken: cfg.offlineToken,
			},
		}
	}

	api, err := gen.NewClientWithResponses(cfg.baseURL, gen.WithHTTPClient(&httpClient))
	if err != nil {
		return nil, fmt.Errorf("swh: %w", err)
	}
	return &Client{API: api, httpClient: &httpClient, baseURL: cfg.baseURL}, nil
}

// Get makes a raw request, for the endpoints the description does not cover
// and for following URLs the archive returns in its responses.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("swh: %w", err)
	}
	// The archive serves HTML documentation from neighbouring paths, and that
	// HTML sits behind a proof-of-work challenge which answers 200.
	request.Header.Set("Accept", "application/json")
	return c.httpClient.Do(request)
}

func transportOf(client *http.Client) http.RoundTripper {
	if client.Transport != nil {
		return client.Transport
	}
	return http.DefaultTransport
}
