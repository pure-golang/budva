// Package support предоставляет общие тестовые хелперы для integration, bdd и e2e слоёв.
//
// Основные компоненты:
//
//   - FakeTelegram — HTTP-сервер, эмулирующий Telegram Bot API.
//   - TestEnv — загрузка тестового окружения и конфигурации.
//   - BadgerContainer — testcontainer с BadgerDB для интеграционных тестов.
//   - RemoteKV — клиент удалённого KV-хранилища для тестов state-репозитория.
package support
