-- +goose Up
CREATE TABLE IF NOT EXISTS conversions (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	format TEXT NOT NULL,
	status TEXT NOT NULL,
	start_time DATETIME NOT NULL,
	end_time DATETIME,
	filename TEXT,
	error TEXT,
	progress REAL DEFAULT 0,
	download_url TEXT
);

-- +goose Down
DROP TABLE IF EXISTS conversions;
