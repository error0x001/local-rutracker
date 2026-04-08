# RuTracker Архив — Поиск по раздачам

Полная база всех раздач RuTracker у вас на компьютере. Офлайн-поиск по миллионам торрентов без интернета, трекеров и блокировок.

## Быстрый старт (Docker)

### 1. Подготовьте архив

Положите файл `rutracker-20260329.xml.xz` в корень проекта.

### 2. Запустите PostgreSQL

```bash
make docker-pg
```

### 3. Загрузите данные

```bash
make docker-migrate
```

Миграция занимает ~1-2 часа. Прогресс: `make docker-logs-migrator`.
При обрыве — просто перезапустите, миграция продолжится с места остановки.

### 4. Запустите сервер

```bash
make docker-server
```

Откройте http://localhost:8080

## Порядок запуска

Важно соблюдать последовательность:

```bash
make docker-pg        # 1. Поднять базу данных
make docker-migrate   # 2. Загрузить данные (дождаться окончания)
make docker-server    # 3. Запустить веб-сервер
```

Остановить: `make docker-down`  
Очистить всё: `make docker-down-v`

## Возможности

- **Полнотекстовый поиск** по заголовкам
- **Фильтрация по категориям** — каскадные dropdown: Категория → Подкатегория → Подподкатегория
- **Рендер BBCode** — спойлеры, цитаты, картинки, шрифты, цвета, смайлики
- **Магнет-ссылки** — корректная генерация с трекером RuTracker
- **Пагинация** — быстрая, без дорогого COUNT(*)
- **JSON API** — `/api/search`, `/api/torrent/{id}`, `/api/categories`

## API

```bash
# Поиск
GET /api/search?q=фильм&cat=Кино&limit=20&offset=0&sort=date

# Получить торрент
GET /api/torrent/123456

# Список категорий
GET /api/categories

# Подкатегории категории
GET /api/subcategories?cat=Игры

# Подподкатегории
GET /api/subsubcategories?cat=Apple&sub=iOS
```

## Локальная разработка (без Docker)

```bash
# Зависимости
go mod download

# Сборка
make build

# Запуск сервера (требует PostgreSQL на порту 5433)
make run-server

# Запуск миграции
make run-migrator
```

## Конфигурация

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `DB_HOST` | localhost | Хост PostgreSQL |
| `DB_PORT` | 5432 | Порт PostgreSQL |
| `DB_USER` | rutracker | Пользователь БД |
| `DB_PASSWORD` | rutracker | Пароль БД |
| `DB_NAME` | rutracker | Имя БД |
| `SERVER_ADDR` | :8080 | Адрес HTTP-сервера |
| `MIGRATOR_PROGRESS` | 10000 | Прогресс миграции каждые N записей |

## Архитектура

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  .xml.xz     │────▶│  Migrator    │────▶│  PostgreSQL  │
│  (4GB+ XML)  │     │ (streaming)  │     │  (tsvector)  │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                 │
┌──────────────┐     ┌──────────────┐            │
│   Браузер    │◀────│ Web Server   │◀───────────┘
│  (HTML/CSS)  │     │ (net/http)   │
└──────────────┘     └──────────────┘
```

## Структура проекта

```
├── cmd/
│   ├── migrator/       # CLI: загрузка XML в PostgreSQL
│   └── server/         # Веб-сервер: поиск и просмотр
│       └── web/        # Шаблоны HTML и статика
├── internal/
│   ├── bbcode/         # Парсер BBCode → HTML
│   ├── config/         # Конфигурация из env
│   ├── db/             # Подключение к БД и миграции
│   ├── migrator/       # Потоковый парсер XML
│   ├── model/          # Модели данных
│   └── search/         # Поиск в PostgreSQL
├── Dockerfile          # Multi-stage build
├── docker-compose.yml  # PostgreSQL + Migrator + Server
├── Makefile
└── README.md
```

## Скачать архив

Магнет-ссылка для загрузки полной базы RuTracker:

```
magnet:?xt=urn:btih:ff805117d5e6258ba71a21bdb9a322cbe0338fa0&dn=rutracker-20260329.xml.xz&tr=http%3A%2F%2Fbt4.t-ru.org%2Fann%3Fmagnet
```

Скачанный файл `rutracker-20260329.xml.xz` положите в корень проекта.
