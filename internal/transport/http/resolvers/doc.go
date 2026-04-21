// Package resolvers содержит имплементацию GraphQL-резолверов для transport/http.
//
// Использование:
//
//	r := resolvers.New(statusService)
//	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: r}))
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Резолверы — тонкая граница: распаковка аргументов, вызов сервиса через частично
//     применяемый интерфейс, возврат результата. Бизнес-логика живёт в сервисах.
//   - Файл `schema.resolvers.go` генерируется gqlgen; новые методы добавляются как стабы
//     после `task gqlgen`.
package resolvers
