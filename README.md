# swh-go

A Go client for the [Software Heritage](https://www.softwareheritage.org/)
Web API.

Software Heritage publishes no OpenAPI description of its API, so the typed
request methods here are generated from one produced by
[swh-openapi](https://github.com/andrew/swh-openapi), which reads swh-web's own
endpoint documentation. `openapi.yaml` is vendored, and `internal/gen` is
generated from it and committed, so building this module needs no code
generation step.

    go get github.com/andrew/swh-go

## Using it

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

`client.API` holds one method per documented endpoint. `examples/search` is
the above as a runnable command.

Response types are named after the route that first returns their shape, and
aliased in this package so callers need not import `internal/gen`. Several
routes share a type where the archive documents them identically.

## What the generated methods do not cover

The description says nothing about pagination, rate limits, authentication or
the archive's asynchronous endpoints. Those are hand written here.

**Pagination** is a `Link` header with `rel="next"`. The query parameters
behind it differ per endpoint, `limit` and `page_token` on origin search
against `per_page` and `last_visit` on origin visits, so this package follows
the header rather than modelling each scheme:

    for response, err := range client.Pages(ctx, url) {
        if err != nil {
            return err
        }
        defer response.Body.Close()
        // ...
    }

`swh.Collect[T](ctx, client, url)` accumulates every element across pages for
endpoints returning JSON arrays. It holds the whole result in memory, so range
over `Pages` for large ones.

**Rate limits** vary by endpoint as well as by whether you are authenticated.
Anonymously the limit is 120 an hour on most routes and 10 an hour on origin
search. `swh.RateLimitOf(response)` reads the headers, and `RetryAfter` gives
the wait once the budget is spent.

**Authentication** raises those limits. Pass the long-lived offline token from
your archive account page:

    client, err := swh.New(swh.WithOfflineToken(os.Getenv("SWH_TOKEN")))

The token is exchanged for short-lived bearer tokens at the Software Heritage
Keycloak realm, cached until shortly before expiry, and exchanged again if the
archive answers 401.

**Vault cooking and Save Code Now** are asynchronous. Both return a status
that the caller polls until it reaches a terminal state. This package does not
wrap that yet.

One operational note: the archive serves HTML documentation from paths next to
the API, and that HTML sits behind a proof-of-work challenge which answers with
a 200 status. `Client.Get` always sends `Accept: application/json` for that
reason, and anything constructing requests by hand should too.

## Updating

    make spec       # fetch the current description from swh-openapi
    make generate   # regenerate internal/gen from it
    make test

The weekly `spec` workflow does this and opens a pull request when the
description has moved. CI fails if the committed client does not match the
committed description.

## Coverage

The description covers the 59 endpoints the archive documents. Three routed
endpoints carry no documentation upstream and so have no generated method:
`/api/1/`, `/api/1/graph/{graph_query}/` and
`/api/1/raw-extrinsic-metadata/get/{id}/`. Reach those with `Client.Get`.

Response field types come from prose docstrings upstream and are one level
deep, so some bodies are less precisely typed than the endpoint deserves. The
corrections live in swh-openapi's overlay rather than here.

## Licence

MIT, in `LICENSE`.

`openapi.yaml` is vendored from swh-openapi and is CC0-1.0.
