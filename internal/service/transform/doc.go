// Package transform применяет текстовые трансформации к сообщениям.
//
// Использование:
//
//	svc := transform.New(telegram, state, logger)
//	text, err := svc.Transform(ctx, domain.TransformParams{...})
//	updated := svc.AddNextLink(ctx, text, src, dstChatID, nextMessageID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Трансформации применяются последовательно: перевод, замена ссылок,
//     замена фрагментов, подпись, ссылка на источник, навигация prev/next.
//   - Зависит от telegramGateway и stateStore через частично применяемые интерфейсы.
package transform
