# swh-go

A Go client for the [Software Heritage](https://www.softwareheritage.org/)
Web API.

Software Heritage documents its API in prose only, so the typed request
methods here are generated from a description produced by
[swh-openapi](https://github.com/andrew/swh-openapi), which reads swh-web's own
endpoint documentation. `openapi.yaml` is vendored, and `internal/gen` is
generated from it and committed, so `go build` is the only step required.

    go get github.com/andrew/swh-go

## Usage

    client, err := swh.New()
    if err != nil {
        return err
    }

    limit := 5
    response, err := client.API.Api1OriginSearchWithResponse(
        ctx, "torvalds/linux", &swh.Api1OriginSearchParams{Limit: &limit},
    )
    if err != nil {
        return err
    }

    for _, origin := range *response.JSON200 {
        fmt.Println(*origin.Url)
    }

`client.API` has one method per documented endpoint. `examples/search` is
the above as a runnable command.

Response types are named after the route that first returns their shape, and
aliased in this package so callers import only `swh`. Several routes share a
type where the upstream documentation describes them identically.

## Hand-written

The description omits pagination, rate limits, authentication and the
archive's asynchronous endpoints, which are hand written here.

Pagination uses a `Link` header with `rel="next"`. The query parameters behind
it differ per endpoint, `limit` and `page_token` on origin search against
`per_page` and `last_visit` on origin visits, so this package reads the `Link`
header for the next URL:

    for response, err := range client.Pages(ctx, url) {
        if err != nil {
            return err
        }
        defer response.Body.Close()
        // ...
    }

`swh.Collect[T](ctx, client, url)` accumulates every element across pages for
endpoints returning JSON arrays. It keeps the whole result in memory, so range
over `Pages` for large ones.

Rate limits vary by endpoint as well as by whether you are authenticated.
Anonymously the limit is 120 an hour on most routes and 10 an hour on origin
search. `swh.RateLimitOf(response)` reads the headers, and `RetryAfter` gives
the wait once the budget is spent.

Authentication raises those limits: pass the long-lived offline token from
your archive account page.

    client, err := swh.New(swh.WithOfflineToken(os.Getenv("SWH_TOKEN")))

The token is exchanged for short-lived bearer tokens at the Software Heritage
Keycloak realm and cached until shortly before expiry. A 401 from the archive
triggers another exchange.

Vault cooking and Save Code Now are asynchronous. Both return a status that
the caller polls until it reaches a terminal state, and polling is left to the
caller.

The archive serves HTML documentation from paths next to the API, and that
HTML is behind a proof-of-work challenge that returns a 200 status.
`Client.Get` always sends `Accept: application/json` for that reason.
Hand-built requests must set the same header.

## Updating

    make spec       # fetch the current description from swh-openapi
    make generate   # regenerate internal/gen from it
    make test

The weekly `spec` workflow does this and opens a pull request when the
description has changed. CI requires the committed client to match the
committed description.

## Coverage

The description covers the 59 documented endpoints. Three routed endpoints are
undocumented upstream, so `internal/gen` omits them:
`/api/1/`, `/api/1/graph/{graph_query}/` and
`/api/1/raw-extrinsic-metadata/get/{id}/`. Reach those with `Client.Get`.

Response field types come from prose docstrings upstream and are one level
deep, so some bodies are typed more loosely than the response shape allows.
The corrections are in swh-openapi's overlay.

## Licence

MIT, in `LICENSE`. `openapi.yaml` is vendored from swh-openapi under
CC0-1.0.
