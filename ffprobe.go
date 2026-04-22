package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type ffprobeField int

const (
	ffprobeVideoPixelFormat ffprobeField = iota
	ffprobeAudioBitrate
	ffprobeDuration
	ffprobeVideoWidth
	ffprobeVideoBitrate
)

var ffprobeOutput = func(input string, field ffprobeField) (string, error) {
	args := []string{
		"-v", "error",
		"-of", "csv=p=0",
	}

	switch field {
	case ffprobeVideoPixelFormat:
		args = append(args, "-select_streams", "v:0", "-show_entries", "stream=pix_fmt")
	case ffprobeAudioBitrate:
		args = append(args, "-select_streams", "a:0", "-show_entries", "stream=bit_rate")
	case ffprobeDuration:
		args = append(args, "-show_entries", "format=duration")
	case ffprobeVideoWidth:
		args = append(args, "-select_streams", "v:0", "-show_entries", "stream=width")
	case ffprobeVideoBitrate:
		args = append(args, "-select_streams", "v:0", "-show_entries", "stream=bit_rate")
	default:
		return "", fmt.Errorf("unsupported ffprobe field: %d", field)
	}

	args = append(args, input)
	cmd := exec.Command("ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ffprobe failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
