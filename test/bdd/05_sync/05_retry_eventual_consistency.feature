Feature: Повторная попытка при eventual consistency

  Как система пересылки
  Я хочу повторять операции если permanent ID ещё не записан
  Чтобы eventual consistency не приводила к потере синхронизации

  Scenario: 01_deletion_is_retried_if_permanent_id_is_missing_in_storage
    Given правило пересылки в режиме "копия"
    And пользователь ранее отправил сообщение
    And сообщение было скопировано в целевые чаты
    And permanent ID ещё не записан в хранилище
    When пользователь удаляет оригинальное сообщение
    And permanent ID записывается в хранилище
    Then после повторной попытки копии удаляются из целевых чатов
