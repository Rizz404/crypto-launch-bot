-- +goose Up
-- The single dashboard account. The boolean key allows at most one row, so a
-- second registration fails on conflict instead of adding another owner.
CREATE TABLE web_owner (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    email text NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE web_owner;
