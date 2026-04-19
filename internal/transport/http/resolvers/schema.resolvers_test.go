package resolvers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dtogql "github.com/pure-golang/budva-claude/internal/dto/graphql"
	"github.com/pure-golang/budva-claude/internal/transport/http/graph"
	"github.com/pure-golang/budva-claude/internal/transport/http/resolvers"
	"github.com/pure-golang/budva-claude/internal/transport/http/resolvers/mocks"
)

func TestQueryResolver_Status_success(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := mocks.NewStatusService(t)
	svc.EXPECT().GetStatus(mock.Anything).
		Return(&dtogql.StatusResponse{TDLibVersion: "1.8.0", UserID: 12345}, nil)

	r := resolvers.New(svc)

	// Act
	got, err := r.Query().Status(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.8.0", got.TDLibVersion)
	assert.Equal(t, int64(12345), got.UserID)
}

func TestQueryResolver_Status_error(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := mocks.NewStatusService(t)
	svc.EXPECT().GetStatus(mock.Anything).
		Return(nil, errors.New("telegram unavailable"))

	r := resolvers.New(svc)

	// Act
	got, err := r.Query().Status(context.Background())

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "telegram unavailable")
	assert.Nil(t, got)
}

// Проверка реализации graph.ResolverRoot — ловит регрессию сигнатур при смене схемы.
var _ graph.ResolverRoot = (*resolvers.Resolver)(nil)
