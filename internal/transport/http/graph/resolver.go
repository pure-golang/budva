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
	statusProvider statusProvider
}

// NewResolver создаёт новый resolver.
func NewResolver(statusProvider statusProvider) *Resolver {
	return &Resolver{statusProvider: statusProvider}
}
