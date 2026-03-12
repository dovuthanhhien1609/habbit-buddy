-- habit-buddy initial schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS habits (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '#6366f1',
    icon        TEXT NOT NULL DEFAULT 'check',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS habit_completions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    habit_id       UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    completed_date DATE NOT NULL,
    completed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(habit_id, completed_date)
);

CREATE INDEX IF NOT EXISTS idx_completions_habit_date
    ON habit_completions(habit_id, completed_date DESC);

CREATE INDEX IF NOT EXISTS idx_completions_user_date
    ON habit_completions(user_id, completed_date DESC);

CREATE INDEX IF NOT EXISTS idx_habits_user_active
    ON habits(user_id) WHERE is_active = TRUE;
