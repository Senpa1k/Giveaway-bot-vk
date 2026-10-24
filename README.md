# Giveaway-bot-vk

Чат-бот сообщества VK для ежедневных розыгрышей партнеров и финального
розыгрыша главного приза. Go + PostgreSQL, архитектура и код-стайл перенесены
из проекта GorkyCode: слои `http-server/handlers` → `service` → `storage`,
миграции goose, единый golangci-lint, Makefile.

## Что умеет

- **Запуск** — приветствие с баннером и главным меню: «Получить промокод»,
  «Участвовать в розыгрыше», «Как участвовать», «Мои шансы», «Правила акции».
- **Промокоды** — уникальный код партнера дня с инструкцией; повторное
  нажатие в тот же день показывает закрепленный код; когда коды закончились,
  участие в розыгрыше остается доступным.
- **Ежедневный розыгрыш** — обязательная подписка на аккаунт партнера,
  автоматическая проверка `groups.isMember` с неограниченными повторами,
  одна заявка на пользователя в день.
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

1. `cp .env.example .env` и заполните `VK_TOKEN`, `VK_GROUP_ID`, `ADMIN_TOKEN`.
2. `docker compose up --build -d`.
3. Примените миграции: `make migrate` (из корня, `DATABASE_URL` берется из
   `.env`; goose скачается через `go run`, если не установлен).
4. Создайте партнера и загрузите промокоды через admin API (примеры ниже).
5. Локальная проверка: `http://localhost:8080/health` и `/ready`.

Запуск вне Docker: `docker compose up -d postgres`, затем `make run`.

## Настройка сообщества VK

1. Управление сообществом → «Сообщения» → включить сообщения сообщества,
   настроить кнопку «Начать».
2. «Работа с API» → «Ключи доступа» → создать ключ с правами
   `messages`, `groups` — это `VK_TOKEN`.
3. **Режим longpoll** (`VK_MODE=longpoll`) — для локальной разработки: бот сам
   забирает события, публичный адрес не нужен. Укажите `VK_GROUP_ID`.
4. **Режим callback** (`VK_MODE=callback`) — для продакшена: в разделе
   «Callback API» укажите `https://<ваш-домен>/callback`, скопируйте строку
   подтверждения в `VK_CONFIRMATION` и задайте `VK_SECRET` (бот проверяет
   секрет каждого события).

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `DATABASE_URL` | строка подключения PostgreSQL |
| `VK_MODE` | `longpoll` или `callback` |
| `VK_GROUP_ID` | ID сообщества (нужен в longpoll) |
| `VK_TOKEN` | ключ доступа сообщества |
| `VK_CONFIRMATION` / `VK_SECRET` | строка подтверждения и секрет Callback API |
| `ADMIN_TOKEN` | bearer-токен admin API |
| `CAMPAIGN_START_DATE` / `CAMPAIGN_END_DATE` | сроки акции (`YYYY-MM-DD`) |
| `CAMPAIGN_MAIN_PRIZE` | текст главного приза |
| `CAMPAIGN_MAX_CHANCES` | потолок шансов (по умолчанию 10) |
| `DRAW_TIME` / `DRAW_TIMEZONE` | время ежедневного розыгрыша |
| `DRAW_FINAL_WINNERS` / `DRAW_TICKETS_PER_WINNER` / `DRAW_RESERVE_COUNT` | параметры финала |
| `DRAW_AUTO_FINAL` | запускать финальный розыгрыш автоматически в последний день |
| `BANNER` | вложение-баннер приветствия, например `photo-123_456` |

Полный список значений по умолчанию — в `config/local.yaml`.

## Admin API

Все запросы требуют заголовок `Authorization: Bearer <ADMIN_TOKEN>`.
Контракт — `contracts/admin.openapi.yaml`.

```bash
# Партнер дня
curl -X POST http://localhost:8080/api/admin/partners \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Партнер","day":"2026-11-01","vk_group_id":123,"vk_url":"https://vk.com/partner",
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
internal/vkapi/     клиент VK API, клавиатуры, callback/longpoll события
internal/service/   сценарии: бот, промокоды, участие, шансы, розыгрыши,
                    победители, статистика, рассылка
internal/scheduler/ автоматический запуск розыгрышей
internal/http-server/  callback VK + admin API + health
```

Обработчики зависят от интерфейсов сервисов, сервисы — от интерфейсов
репозиториев; реальные типы собираются в `storage.NewRepository` и
`service.New` (как в GorkyCode).
