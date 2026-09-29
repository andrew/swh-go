package swh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNextPageReadsTheLinkHeader(t *testing.T) {
	response := &http.Response{Header: http.Header{}}
	response.Header.Set("Link", `<https://example.test/api/1/x/?page_token=abc>; rel="next"`)

	if got, want := NextPage(response), "https://example.test/api/1/x/?page_token=abc"; got != want {
		t.Errorf("NextPage() = %q, want %q", got, want)
	}
}

func TestNextPageIgnoresOtherRelations(t *testing.T) {
	response := &http.Response{Header: http.Header{}}
	response.Header.Set("Link", `<https://example.test/first>; rel="previous"`)

	if got := NextPage(response); got != "" {
		t.Errorf("NextPage() = %q, want empty", got)
	}
}

func TestNextPageOnTheLastPage(t *testing.T) {
	if got := NextPage(&http.Response{Header: http.Header{}}); got != "" {
		t.Errorf("NextPage() = %q, want empty", got)
	}
}

func TestPagesFollowsEveryLink(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" {
			page = "1"
		}
		if page != "3" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/?page=%s>; rel="next"`, server.URL, next(page)))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"page":%s}]`, page)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	type row struct {
		Page int `json:"page"`
	}
	rows, err := Collect[row](context.Background(), client, server.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("collected %d rows, want 3", len(rows))
	}
	for i, r := range rows {
		if r.Page != i+1 {
			t.Errorf("row %d is page %d, want %d", i, r.Page, i+1)
		}
	}
}

func next(page string) string {
	switch page {
	case "1":
		return "2"
	default:
		return "3"
	}
}

func TestRateLimitOf(t *testing.T) {
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	response := &http.Response{Header: http.Header{}}
	response.Header.Set("X-RateLimit-Limit", "10")
	response.Header.Set("X-RateLimit-Remaining", "0")
	response.Header.Set("X-RateLimit-Reset", fmt.Sprint(reset.Unix()))

	limit, present := RateLimitOf(response)
	if !present {
		t.Fatal("RateLimitOf() reported no headers")
	}
	if limit.Limit != 10 || limit.Remaining != 0 {
		t.Errorf("got %+v, want limit 10 remaining 0", limit)
	}
	if !limit.Reset.Equal(reset) {
		t.Errorf("Reset = %v, want %v", limit.Reset, reset)
	}
	if wait := limit.RetryAfter(time.Now()); wait <= 0 {
		t.Errorf("RetryAfter() = %v, want positive", wait)
	}
}

func TestRateLimitOfWithoutHeaders(t *testing.T) {
	if _, present := RateLimitOf(&http.Response{Header: http.Header{}}); present {
		t.Error("RateLimitOf() reported headers on a bare response")
	}
}

func TestRetryAfterIsZeroWhileRequestsRemain(t *testing.T) {
	limit := RateLimit{Remaining: 5, Reset: time.Now().Add(time.Hour)}
	if wait := limit.RetryAfter(time.Now()); wait != 0 {
		t.Errorf("RetryAfter() = %v, want 0", wait)
	}
}

func TestOfflineTokenIsExchangedAndSent(t *testing.T) {
	var exchanges int
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges++
		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.FormValue("refresh_token"); got != "offline-token" {
			t.Errorf("refresh_token = %q, want offline-token", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "bearer-token", "expires_in": 300,
		})
	}))
	defer auth.Close()

	var seen string
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer archive.Close()

	client, err := New(
		WithBaseURL(archive.URL),
		WithOfflineToken("offline-token"),
		WithAuthEndpoint(auth.URL, "swh-web"),
	)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		response, err := client.Get(context.Background(), archive.URL+"/api/1/stat/counters/")
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
	}

	if seen != "Bearer bearer-token" {
		t.Errorf("Authorization = %q, want %q", seen, "Bearer bearer-token")
	}
	// The second request reuses the cached token rather than exchanging again.
	if exchanges != 1 {
		t.Errorf("exchanged the offline token %d times, want 1", exchanges)
	}
}

func TestExpiredTokenIsExchangedAgainOn401(t *testing.T) {
	var exchanges int
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", exchanges), "expires_in": 300,
		})
	}))
	defer auth.Close()

	var attempts int
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer archive.Close()

	client, err := New(
		WithBaseURL(archive.URL),
		WithOfflineToken("offline-token"),
		WithAuthEndpoint(auth.URL, "swh-web"),
	)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Get(context.Background(), archive.URL+"/api/1/stat/counters/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %s, want 200 after re-exchanging", response.Status)
	}
	if exchanges != 2 {
		t.Errorf("exchanged the offline token %d times, want 2", exchanges)
	}
}

func TestGetAsksForJSON(t *testing.T) {
	var accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(context.Background(), server.URL+"/api/1/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", accept)
	}
}
