package swh

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"regexp"
)

// The archive paginates with a Link header. The query parameters behind it
// differ between endpoints, origin search using limit and page_token where
// origin visits uses per_page and last_visit, so following the header is the
// only way to page uniformly.
var nextLink = regexp.MustCompile(`<([^>]+)>\s*;\s*rel="next"`)

// NextPage returns the URL of the page after the one that produced this
// response, or an empty string on the last page.
func NextPage(response *http.Response) string {
	match := nextLink.FindStringSubmatch(response.Header.Get("Link"))
	if match == nil {
		return ""
	}
	return match[1]
}

// Pages walks a paginated endpoint from the given URL, yielding each response
// in turn. The caller closes each body. Iteration stops at the first error and
// at the first response without a next link.
func (c *Client) Pages(ctx context.Context, url string) iter.Seq2[*http.Response, error] {
	return func(yield func(*http.Response, error) bool) {
		for url != "" {
			response, err := c.Get(ctx, url)
			if err != nil {
				yield(nil, err)
				return
			}
			next := NextPage(response)
			if !yield(response, nil) {
				return
			}
			url = next
		}
	}
}

// Collect walks a paginated endpoint returning JSON arrays and accumulates
// every element. It is the convenient path; for large result sets, range over
// [Client.Pages] instead.
func Collect[T any](ctx context.Context, c *Client, url string) ([]T, error) {
	var all []T
	for response, err := range c.Pages(ctx, url) {
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("swh: %s: %s", url, response.Status)
		}
		var page []T
		err = json.NewDecoder(response.Body).Decode(&page)
		_ = response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("swh: decoding page: %w", err)
		}
		all = append(all, page...)
	}
	return all, nil
}
