-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS conversion_jobs (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	format TEXT NOT NULL,
	status TEXT NOT NULL,
	start_time TIMESTAMP NOT NULL,
	end_time TIMESTAMP,
	filename TEXT,
	error TEXT,
	progress REAL DEFAULT 0,
	download_url TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_conversion_jobs_status ON conversion_jobs(status);
CREATE INDEX IF NOT EXISTS idx_conversion_jobs_start_time ON conversion_jobs(start_time DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS conversion_jobs;
-- +goose StatementEnd