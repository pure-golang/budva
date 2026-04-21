package resolvers

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

import (
	"context"

	"github.com/pure-golang/budva-claude/internal/dto"
)

type statusService interface {
	GetStatus(ctx context.Context) (*dto.GraphQLStatusResponse, error)
}

// Resolver связывает GraphQL-схему с сервисным слоем.
type Resolver struct {
	statusService statusService
}

// New создаёт resolver с внедрёнными зависимостями.
func New(statusService statusService) *Resolver {
	return &Resolver{statusService: statusService}
}
