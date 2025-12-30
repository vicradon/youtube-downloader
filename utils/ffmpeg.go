package utils

import "os/exec"

func BuildFFmpegCommand(inputFile, outputFile, format string) *exec.Cmd {
	var args []string

	switch format {
	case "avi":
		args = []string{"-y", "-i", inputFile, "-c:v", "mpeg4", "-c:a", "mp3", outputFile}
	case "mpg":
		args = []string{"-y", "-i", inputFile, "-c:v", "mpeg2video", "-q:v", "2", "-c:a", "mp2", "-b:a", "192k", outputFile}
	default:
		args = []string{"-y", "-i", inputFile, "-c:v", "libx264", "-c:a", "aac", outputFile}
	}

	return exec.Command("ffmpeg", args...)
}
