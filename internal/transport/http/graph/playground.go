package graph

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/playground"
)

// PlaygroundHandler возвращает HTML-страницу GraphQL Playground.
func PlaygroundHandler(endpoint string) http.HandlerFunc {
	return playground.Handler("GraphQL playground", endpoint)
}
