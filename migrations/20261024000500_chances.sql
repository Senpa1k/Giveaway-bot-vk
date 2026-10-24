-- +goose Up
-- +goose StatementBegin
-- Один активный день = один шанс в финальном розыгрыше. Уникальность
-- (user_id, day) не дает начислить два шанса за день: и за промокод, и за
-- регистрацию в розыгрыше.
CREATE TABLE chances (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    day        DATE NOT NULL,
    -- Действие дня: promo или draw.
    source     TEXT NOT NULL CHECK (source IN ('promo', 'draw')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, day)
);

CREATE INDEX chances_day_idx
    ON chances (day);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chances;
-- +goose StatementEnd
