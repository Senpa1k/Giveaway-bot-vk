-- +goose Up
-- +goose StatementBegin
CREATE TABLE promocodes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id   UUID NOT NULL REFERENCES partners(id),
    code         TEXT NOT NULL CHECK (btrim(code) <> ''),
    -- NULL, пока промокод не выдан; выдача фиксирует пользователя и время.
    user_id      UUID REFERENCES users(id),
    assigned_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX promocodes_partner_code_unique
    ON promocodes (partner_id, code);

-- Свободные промокоды партнера для атомарной выдачи.
CREATE INDEX promocodes_free_idx
    ON promocodes (partner_id, created_at)
    WHERE user_id IS NULL;

CREATE INDEX promocodes_user_idx
    ON promocodes (user_id);

-- Журнал нажатий «Получить промокод»: считает запросы даже когда
-- промокоды закончились.
CREATE TABLE promo_requests (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    partner_id UUID NOT NULL REFERENCES partners(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX promo_requests_partner_idx
    ON promo_requests (partner_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS promo_requests;
DROP TABLE IF EXISTS promocodes;
-- +goose StatementEnd
