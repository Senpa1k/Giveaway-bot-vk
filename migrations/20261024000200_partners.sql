-- +goose Up
-- +goose StatementBegin
CREATE TABLE partners (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL CHECK (btrim(name) <> ''),
    -- День партнера: один партнер выступает один день акции.
    day               DATE NOT NULL,
    -- ID сообщества VK для проверки подписки groups.isMember.
    vk_group_id       BIGINT NOT NULL,
    -- Ссылка кнопки «Подписаться на партнера».
    vk_url            TEXT NOT NULL,
    promo_valid_until DATE,
    -- Куда применять промокод: сайт, приложение, точки продаж.
    promo_where       TEXT NOT NULL DEFAULT '',
    prize_name        TEXT NOT NULL,
    -- Инструкция получения приза для сообщения победителю.
    prize_instruction TEXT NOT NULL DEFAULT '',
    winners_count     INT NOT NULL DEFAULT 1 CHECK (winners_count > 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX partners_day_unique
    ON partners (day);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS partners;
-- +goose StatementEnd
