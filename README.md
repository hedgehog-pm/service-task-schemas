# service-task-schemas

Сервис хранит версии схем параметров задач (`rawParams`) в PostgreSQL.
По схеме можно получить её поля вместе с унаследованными, посмотреть список
версий или найти контракты по нужным полям.

В `schemas/` находится только схема `root` версии `1` с полями `title`
и `description`. Команда миграции загружает её в базу. Опубликованные версии
неизменяемы, повторная загрузка того же определения ничего не меняет.

## Запуск

Нужны Go 1.27 и PostgreSQL:

```sh
export DATABASE_URL='postgres://user:pass@localhost:5432/task_schemas?sslmode=disable'
go run ./cmd/migrate
go run ./cmd/service
```

Адрес сервера задаётся в `HTTP_ADDR`, по умолчанию — `:8080`.

## API

- `GET /schemas/root/versions/1` — схема версии `1`.
- `GET /schemas/root` — список версий.
- `GET /schemas?fields=title,description` — контракты с указанными полями.

Полный контракт — в [api/openapi.yaml](api/openapi.yaml).

## Разработка

`make generate` обновляет код из OpenAPI и SQL-запросов, `make fmt` применяет
форматирование, `make lint` проверяет код. Сборка — `go build ./cmd/...`.
Тесты пока не пишем, правило закреплено в [.agents/rules/testing.md](.agents/rules/testing.md).
