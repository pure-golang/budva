// Package state реализует KV-хранилище на основе BadgerDB для хранения состояния пересылки.
//
// Использование:
//
//	r := state.New(cfg, logger)
//	if err := r.Start(ctx); err != nil { ... }
//	defer r.Close()
//
//	err := r.Set("key", "value")
//	val, err := r.Get("key")
//
// Конфигурация:
//
//	STORAGE_PATH — путь к директории BadgerDB (default: .data/badger)
//
// Ограничения:
//
//   - Start() открывает БД и запускает GC-горутину.
//   - Close() должен быть вызван для корректного завершения.
//   - GetSet выполняет атомарное чтение-изменение-запись в одной транзакции.
//   - Потокобезопасен через внутренние транзакции BadgerDB.
package state
