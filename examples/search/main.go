// Command search looks up origins in the Software Heritage archive.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/andrew/swh-go"
)

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		log.Fatalf("usage: %s <url pattern>", os.Args[0])
	}

	client, err := swh.New()
	if err != nil {
		log.Fatal(err)
	}

	limit := 5
	response, err := client.API.Api1OriginSearchWithResponse(
		context.Background(), args[0], &swh.Api1OriginSearchParams{Limit: &limit},
	)
	if err != nil {
		log.Fatal(err)
	}

	if limits, ok := swh.RateLimitOf(response.HTTPResponse); ok {
		fmt.Printf("%d of %d requests left this hour\n\n", limits.Remaining, limits.Limit)
	}

	for _, origin := range *response.JSON200 {
		fmt.Println(*origin.Url)
	}
}
