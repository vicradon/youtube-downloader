-- +goose Up
-- +goose StatementBegin
ALTER TABLE conversion_jobs ADD COLUMN IF NOT EXISTS video_title TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE conversion_jobs DROP COLUMN IF EXISTS video_title;
-- +goose StatementEnd