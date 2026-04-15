// Package graph реализует GraphQL-транспорт поверх HTTP.
//
// Использование:
//
//	resolver := graph.NewResolver(statusProvider)
//	handler := graph.NewHandler(resolver)
//	mux.Handle("/query", handler)
//	mux.Handle("/", graph.PlaygroundHandler("/query"))
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Схема описана в schema.graphqls.
//   - Playground доступен только для отладки.
package graph
