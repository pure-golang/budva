Feature: Rate limiting пересылки

  Как оператор пересылки
  Я хочу ограничить частоту пересылки
  Чтобы не превысить лимиты Telegram API

  Scenario: 01_forwarding_to_one_chat_is_limited_to_once_every_3_seconds
    Given правило пересылки в режиме "копия"
    When пользователь отправляет два сообщения подряд
    Then оба сообщения доставлены в целевые чаты
