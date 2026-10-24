-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    max_id     BIGINT NOT NULL,
    first_name TEXT NOT NULL DEFAULT '',
    last_name  TEXT NOT NULL DEFAULT '',
    -- Состояние диалога для сбора контактов победителя: '', awaiting_name,
    -- awaiting_consent. Контекст состояния хранится в state_ctx.
    state      TEXT NOT NULL DEFAULT '',
    state_ctx  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_max_id_unique
    ON users (max_id);

-- Статистика новых пользователей по дням.
CREATE INDEX users_created_at_idx
    ON users (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
