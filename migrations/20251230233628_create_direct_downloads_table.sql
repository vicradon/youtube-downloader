-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS direct_downloads (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	filename TEXT NOT NULL,
	download_time TIMESTAMP NOT NULL,
	status TEXT NOT NULL DEFAULT 'processing',
	error TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_direct_downloads_status ON direct_downloads(status);
CREATE INDEX IF NOT EXISTS idx_direct_downloads_download_time ON direct_downloads(download_time DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS direct_downloads;
-- +goose StatementEnd
