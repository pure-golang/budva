// Package graphql содержит сгенерированный gqlgen контракт GraphQL-схемы.
//
// Использование:
//
//	cfg := graphql.Config{Resolvers: resolver}
//	schema := graphql.NewExecutableSchema(cfg)
//
// Конфигурация:
//
//	Пакет не читает переменные окружения напрямую.
//
// Пакет играет ту же роль, что grpc/pb/: хранит `.graphqls`-схему и generated exec-код,
// читается только автогенератором. Правится руками только в части `schema.graphqls`;
// остальные файлы перезаписываются при `task gqlgen`.
//
// Имплементация резолверов живёт в `internal/transport/http/resolvers/`.
//
// Ограничения:
//
//   - Руками правится только `schema.graphqls` и package comment в `doc.go`.
//   - Остальные файлы пакета перезаписываются при `task gqlgen`.
package graphql
