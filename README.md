# YouTube Video Downloader

A simple HTTP-based YouTube video downloader with format conversion capabilities.

## Features

- Download YouTube videos directly or convert to different formats (MPG, AVI)
- Real-time conversion status tracking
- SQLite persistence for conversion history
- Retry failed conversions
- Geist-style UI with dark/light theme support

## Setup

### Prerequisites

- Go 1.21+
- FFmpeg installed and available in PATH
- RapidAPI key for YouTube video downloader API

### Environment Variables

Create a `.env` file:

```bash
RAPIDAPI_KEY=your_rapidapi_key_here
EXEC_DIR=/path/to/your/project/directory
```

### Database Migrations

Install [Goose](https://github.com/pressly/goose):

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Run migrations:

```bash
goose sqlite ./app.db up
```

Create a new migration:

```bash
goose -dir migrations create add_new_column sql
```

### Running

```bash
# Run the server
go run main.go

# Or build and run
go build -o viddownloader
./viddownloader
```

Server will start on http://0.0.0.0:8080

## Testing

Run unit tests:

```bash
# Test video conversion (requires nnfs-7.mp4 in project root)
go test -v -run TestVideoConversion

# Test video ID extraction
go test -v -run TestExtractVideoID

# Run all tests
go test -v
```

## API Endpoints

- `POST /download` - Start a new conversion/download
- `GET /conversions` - List all conversions
- `GET /file/{filename}` - Download converted file
- `DELETE /delete/{filename}` - Delete converted file
- `POST /retry/{jobId}` - Retry failed conversion

## Directory Structure

```
.
├── main.go              # Application entry point
├── main_test.go         # Unit tests
├── app.db               # SQLite database (created after migrations)
├── conversions/         # Conversion files
│   ├── ongoing/         # Temporary files during conversion
│   └── completed/       # Final converted files
├── migrations/          # Goose migrations
│   └── 00001_create_conversions_table.sql
├── templates/           # HTML templates
│   └── index.html
└── .env                 # Environment variables
```
