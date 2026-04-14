package graph

import (
	"context"

	dtogql "github.com/pure-golang/budva-claude/internal/dto/graphql"
)

type statusProvider interface {
	GetStatus(ctx context.Context) (*dtogql.StatusResponse, error)
}

// Resolver связывает GraphQL-схему с сервисным слоем.
type Resolver struct {
	status statusProvider
}

// NewResolver создаёт новый resolver.
func NewResolver(status statusProvider) *Resolver {
	return &Resolver{status: status}
}
