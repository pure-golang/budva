// Package transform применяет текстовые трансформации к сообщениям.
//
// Использование:
//
//	svc := transform.New(telegramRepo, stateRepo)
//	text, err := svc.Transform(ctx, domain.TransformParams{Text: ft, Source: src, ...})
//	updated := svc.AddNextLink(ctx, text, src, dstChatID, nextMessageID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Трансформации применяются последовательно: перевод, auto-answer,
//     замена ссылок, замена фрагментов, подпись, ссылка на источник, навигация prev/next.
//   - Зависит от telegramRepo и stateRepo через частично применяемые интерфейсы.
package transform
