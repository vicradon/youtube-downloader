package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVideoConversion(t *testing.T) {
	// Paths to test files
	testVideo := "input.mp4"
	outputDir := "test_output"

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Test if test video exists
	if _, err := os.Stat(testVideo); os.IsNotExist(err) {
		t.Skip("Test video file not found, skipping test")
	}

	tests := []struct {
		name       string
		format     string
		videoCodec string
		audioCodec string
		outputExt  string
	}{
		{
			name:       "Convert to MPG",
			format:     "mpg",
			videoCodec: "mpeg2video",
			audioCodec: "mp2",
			outputExt:  "mpg",
		},
		{
			name:       "Convert to AVI",
			format:     "avi",
			videoCodec: "mpeg4",
			audioCodec: "mp3",
			outputExt:  "avi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use unique output filename per test to avoid conflicts
			outputFile := filepath.Join(outputDir, tt.name+"."+tt.outputExt)

			// Use the actual buildFFmpegCommand function from main.go
			cmd := buildFFmpegCommand(testVideo, outputFile, tt.format)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("FFmpeg conversion failed: %v\nOutput: %s", err, string(output))
			}

			// Check if output file was created
			info, err := os.Stat(outputFile)
			if err != nil {
				t.Fatalf("Output file not created: %v", err)
			}

			// Check if file has content
			if info.Size() == 0 {
				t.Error("Output file is empty")
			}

			t.Logf("Successfully converted to %s: %s (%d bytes)", tt.format, outputFile, info.Size())
		})
	}
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name:    "Standard YouTube URL",
			url:     "https://www.youtube.com/watch?v=dEXPMQXoiLc",
			want:    "dEXPMQXoiLc",
			wantErr: false,
		},
		{
			name:    "Short YouTube URL",
			url:     "https://youtu.be/dEXPMQXoiLc",
			want:    "dEXPMQXoiLc",
			wantErr: false,
		},
		{
			name:    "YouTube URL with parameters",
			url:     "https://www.youtube.com/watch?v=dEXPMQXoiLc&t=10s",
			want:    "dEXPMQXoiLc",
			wantErr: false,
		},
		{
			name:    "Invalid URL",
			url:     "https://example.com/video",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractVideoID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVideoID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractVideoID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"Zero bytes", 0, "0 Bytes"},
		{"Bytes", 512, "512.0 Bytes"},
		{"KB", 1024, "1.0 KB"},
		{"MB", 1048576, "1.0 MB"},
		{"GB", 1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFileSize(tt.bytes); got != tt.want {
				t.Errorf("formatFileSize(%d) = %v, want %v", tt.bytes, got, tt.want)
			}
		})
	}
}
