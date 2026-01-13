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

func BuildCutCommand(inputFile, outputFile, start, end string) *exec.Cmd {
	// Using re-encoding for precision
	// -ss before -i is faster but less accurate for seeking, but since we are re-encoding,
	// placing it before -i is fine for "fast seek" and then re-encoding guarantees frames.
	// Actually, for precise cutting, -ss after -i is slower but frame-perfect.
	// Let's use -ss before -i for speed, as it's usually "good enough" for user snippets,
	// and re-encoding handles the keyframe alignment.
	// "ffmpeg -y -ss start -to end -i input ..."
	
	args := []string{
		"-y", 
		"-ss", start,
		"-to", end,
		"-i", inputFile, 
		"-c:v", "libx264", 
		"-c:a", "aac", 
		outputFile,
	}
	return exec.Command("ffmpeg", args...)
}
