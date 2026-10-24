# Giveaway-bot (MAX)

Чат-бот мессенджера MAX для ежедневных розыгрышей партнеров и финального
розыгрыша главного приза. Go + PostgreSQL, архитектура и код-стайл перенесены
из проекта GorkyCode: слои `http-server/handlers` → `service` → `storage`,
миграции goose, единый golangci-lint, Makefile.

## Что умеет

- **Запуск** — приветствие и главное меню: «Получить промокод»,
  «Участвовать в розыгрыше», «Как участвовать», «Мои шансы», «Правила акции».
- **Промокоды** — уникальный код партнера дня с инструкцией; повторное
  нажатие в тот же день показывает закрепленный код; когда коды закончились,
  участие в розыгрыше остается доступным.
- **Ежедневный розыгрыш** — обязательная подписка на канал партнера,
  автоматическая проверка `GET /chats/{id}/members` с неограниченными
  повторами, одна заявка на пользователя в день.
- **Шансы** — один активный день (промокод или регистрация) = один шанс в
  финале, максимум 10; раздел «Мои шансы» с прогрессом и партнерами.
- **Розыгрыши** — ежедневный запускается автоматически по расписанию;
  финальный взвешен по шансам (каждый шанс — билет), 5 победителей по
  2 билета и резервные победители; результаты подтверждаются
  администратором до отправки уведомлений.
- **Победители** — уведомления, сбор имени/фамилии и согласия на передачу
  данных, передача приза резервному победителю при срыве срока.
- **Аналитика** — пользователи по дням, запросы и выдачи промокодов, остатки
  по партнерам, участие в розыгрышах; выгрузки CSV для команды MAX и партнера.
- **Пуш-уведомления** — рассылка произвольного сообщения всем пользователям.

## Быстрый старт

Нужны Docker и Docker Compose.

1. Зарегистрируйте бота на [dev.max.ru](https://dev.max.ru) и получите
   access token (переменная `MAX_ACCESS_TOKEN`).
2. `cp .env.example .env` и заполните `MAX_ACCESS_TOKEN`, `ADMIN_TOKEN`.
3. `docker compose up --build -d`.
4. Примените миграции: `make migrate` (из корня, `DATABASE_URL` берется из
   `.env`; goose скачается через `go run`, если не установлен).
5. Создайте партнера и загрузите промокоды через admin API (примеры ниже).
6. Локальная проверка: `http://localhost:8080/health` и `/ready`.

Запуск вне Docker: `docker compose up -d postgres`, затем `make run`.

## Настройка бота в MAX

1. Создайте бота в кабинете [dev.max.ru](https://dev.max.ru), включите
   сообщения бота и кнопку «Начать» — скопируйте **access token**
   в `MAX_ACCESS_TOKEN`.
2. Добавьте бота **администратором** в каналы партнеров: проверка подписки
   `GET /chats/{chat_id}/members?user_ids=...` работает только с правами
   администратора.
3. **Режим longpoll** (`MAX_MODE=longpoll`) — для локальной разработки: бот
   сам забирает события через `GET /updates`, публичный адрес не нужен.
4. **Режим webhook** (`MAX_MODE=webhook`) — для продакшена: поднимите бот с
   публичным HTTPS-адресом и подпишите его на события:

   ```bash
   curl -X POST https://platform-api2.max.ru/subscriptions \
     -H "Authorization: $MAX_ACCESS_TOKEN" -H "Content-Type: application/json" \
     -d '{"url":"https://<ваш-домен>/callback","update_types":["bot_started","message_created","message_callback"]}'
   ```

Все запросы к Bot API идут на `platform-api2.max.ru` (домен
`botapi.max.ru` отключен) с токеном в заголовке `Authorization`.
Лимит платформы: не более двух сообщений в секунду в диалог — клиент бота
соблюдает его автоматически.

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `DATABASE_URL` | строка подключения PostgreSQL |
| `MAX_MODE` | `longpoll` или `webhook` |
| `MAX_ACCESS_TOKEN` | access token бота из dev.max.ru |
| `MAX_API_URL` | базовый URL Bot API (по умолчанию `https://platform-api2.max.ru`) |
| `ADMIN_TOKEN` | bearer-токен admin API |
| `CAMPAIGN_START_DATE` / `CAMPAIGN_END_DATE` | сроки акции (`YYYY-MM-DD`) |
| `CAMPAIGN_MAIN_PRIZE` | текст главного приза |
| `CAMPAIGN_MAX_CHANCES` | потолок шансов (по умолчанию 10) |
| `DRAW_TIME` / `DRAW_TIMEZONE` | время ежедневного розыгрыша |
| `DRAW_FINAL_WINNERS` / `DRAW_TICKETS_PER_WINNER` / `DRAW_RESERVE_COUNT` | параметры финала |
| `DRAW_AUTO_FINAL` | запускать финальный розыгрыш автоматически в последний день |

Полный список значений по умолчанию — в `config/local.yaml`.

## Admin API

Все запросы требуют заголовок `Authorization: Bearer <ADMIN_TOKEN>`.
Контракт — `contracts/admin.openapi.yaml`.

```bash
# Партнер дня
curl -X POST http://localhost:8080/api/admin/partners \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Партнер","day":"2026-11-01","max_channel_id":123,"max_url":"https://max.ru/partner",
       "promo_valid_until":"2026-11-10","promo_where":"сайте partner.ru",
       "prize_name":"мерч-пакет","prize_instruction":"Забери приз на стойке.","winners_count":2}'

# Загрузка промокодов
curl -X POST http://localhost:8080/api/admin/partners/<ID>/promocodes \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{"codes":["FEST-001","FEST-002"]}'

# Запуск розыгрышей (день по умолчанию — сегодняшний)
curl -X POST http://localhost:8080/api/admin/draws/daily -H "Authorization: Bearer $ADMIN_TOKEN" -d '{}'
curl -X POST http://localhost:8080/api/admin/draws/final -H "Authorization: Bearer $ADMIN_TOKEN" -d '{"day":"2026-11-10"}'

# Подтверждение результатов — после него уходят уведомления победителям
curl -X POST http://localhost:8080/api/admin/draws/<ID>/approve \
  -H "Authorization: Bearer $ADMIN_TOKEN" -d '{"approved_by":"max.team"}'

# Выгрузки и статистика
curl -H "Authorization: Bearer $ADMIN_TOKEN" "http://localhost:8080/api/admin/draws/<ID>/export?type=winners"
curl -H "Authorization: Bearer $ADMIN_TOKEN" "http://localhost:8080/api/admin/draws/<ID>/export?type=participants"
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/api/admin/stats

# Победитель не ответил — приз резерву
curl -X POST http://localhost:8080/api/admin/winners/<ID>/miss -H "Authorization: Bearer $ADMIN_TOKEN"

# Рассылка всем пользователям
curl -X POST http://localhost:8080/api/admin/broadcast \
  -H "Authorization: Bearer $ADMIN_TOKEN" -d '{"text":"Завтра новый партнер!"}'
```

## Схема работы розыгрышей

Статусная машина розыгрыша: `pending` → `drawn` (случайный выбор сохранен) →
`approved` (администратор подтвердил) → `notified` (победители получили
сообщения). Ежедневный розыгрыш запускается планировщиком в `DRAW_TIME`;
уведомления отправляются только после подтверждения. Повторный запуск
розыгрыша невозможен — переход статусов guarded в PostgreSQL.

## Схема базы данных

`migrations/`: `users` (включая состояние диалога), `partners` (партнер дня),
`promocodes` + `promo_requests` (выдача и журнал запросов), `draws` /
`draw_entries` / `winners` (розыгрыши, участники, победители с контактами),
`chances` (активные дни; уникальность `user_id + day` гарантирует один шанс
в день).

## Разработка

```bash
make help        # список команд
make lint        # линтер и форматирование
make test        # тесты с race detector и покрытием
make run         # запуск с настройками из .env
make migrate     # миграции goose
```

Структура:

```
cmd/bot/            точка входа
internal/config/    настройки (cleanenv, YAML + env)
internal/model/     доменные сущности и sentinel-ошибки
internal/storage/   postgres-репозитории (pgx + squirrel)
internal/maxapi/    клиент MAX Bot API, клавиатуры, события, long polling
internal/service/   сценарии: бот, промокоды, участие, шансы, розыгрыши,
                    победители, статистика, рассылка
internal/scheduler/ автоматический запуск розыгрышей
internal/http-server/  webhook MAX + admin API + health
```

Обработчики зависят от интерфейсов сервисов, сервисы — от интерфейсов
репозиториев; реальные типы собираются в `storage.NewRepository` и
`service.New` (как в GorkyCode).

> Имя репозитория историческое: проект начинался как бот VK, но целевая
> платформа — мессенджер MAX.
