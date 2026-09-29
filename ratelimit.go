package swh

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimit is what the archive reported about the caller's remaining budget.
//
// The limit varies by endpoint as well as by whether the caller is
// authenticated: anonymously it is 120 an hour on most routes and 10 an hour
// on origin search.
type RateLimit struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// RateLimitOf reads the rate limit headers from a response. Present reports
// whether the response carried them at all.
func RateLimitOf(response *http.Response) (limit RateLimit, present bool) {
	values := map[string]int{}
	for _, name := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		raw := response.Header.Get(name)
		if raw == "" {
			return RateLimit{}, false
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return RateLimit{}, false
		}
		values[name] = parsed
	}
	return RateLimit{
		Limit:     values["X-RateLimit-Limit"],
		Remaining: values["X-RateLimit-Remaining"],
		Reset:     time.Unix(int64(values["X-RateLimit-Reset"]), 0),
	}, true
}

// RetryAfter is how long to wait before the budget refills, or zero if
// requests remain.
func (r RateLimit) RetryAfter(now time.Time) time.Duration {
	if r.Remaining > 0 {
		return 0
	}
	if wait := r.Reset.Sub(now); wait > 0 {
		return wait
	}
	return 0
}
