-- +goose Up
-- +goose StatementBegin
CREATE TABLE draws (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind          TEXT NOT NULL CHECK (kind IN ('daily', 'final')),
    -- NULL для финального розыгрыша.
    partner_id    UUID REFERENCES partners(id),
    draw_date     DATE NOT NULL,
    -- pending -> drawn -> approved -> notified.
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'drawn', 'approved', 'notified')),
    winners_count INT NOT NULL DEFAULT 0,
    reserve_count INT NOT NULL DEFAULT 0,
    drawn_at      TIMESTAMPTZ,
    approved_at   TIMESTAMPTZ,
    approved_by   TEXT NOT NULL DEFAULT '',
    notified_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Один ежедневный и один финальный розыгрыш на дату.
CREATE UNIQUE INDEX draws_kind_date_unique
    ON draws (kind, draw_date);

CREATE INDEX draws_partner_idx
    ON draws (partner_id);

-- Итоговый список участников каждого розыгрыша.
CREATE TABLE draw_entries (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id    UUID NOT NULL REFERENCES draws(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (draw_id, user_id)
);

CREATE INDEX draw_entries_draw_idx
    ON draw_entries (draw_id);

CREATE TABLE winners (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id        UUID NOT NULL REFERENCES draws(id),
    user_id        UUID NOT NULL REFERENCES users(id),
    position       INT NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('winner', 'reserve')),
    prize          TEXT NOT NULL,
    -- Билетов на победителя: 2 в финальном розыгрыше по ТЗ.
    tickets        INT NOT NULL DEFAULT 1,
    -- awaiting_response -> contacted -> confirmed;
    -- missed и reassigned - приз не получен вовремя.
    status         TEXT NOT NULL DEFAULT 'awaiting_response'
                   CHECK (status IN ('awaiting_response', 'contacted', 'confirmed', 'missed', 'reassigned')),
    contact_name   TEXT NOT NULL DEFAULT '',
    consent_given  BOOLEAN NOT NULL DEFAULT FALSE,
    consent_at     TIMESTAMPTZ,
    notified_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (draw_id, user_id)
);

CREATE INDEX winners_draw_idx
    ON winners (draw_id);

CREATE INDEX winners_user_idx
    ON winners (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS winners;
DROP TABLE IF EXISTS draw_entries;
DROP TABLE IF EXISTS draws;
-- +goose StatementEnd
