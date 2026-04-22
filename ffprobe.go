package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type ffprobeField int

const (
	ffprobeVideoPixelFormat ffprobeField = iota
	ffprobeAudioBitrate
	ffprobeDuration
	ffprobeVideoWidth
	ffprobeVideoBitrate
	ffprobeOutAudio
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
	case ffprobeOutAudio:
		args = append(args, "-select_streams", "a:0", "-show_entries", "packet=size")
	default:
		return "", fmt.Errorf("unsupported ffprobe field: %d", field)
	}

	args = append(args, input)

	res, err := runFfprobe(args...)
	if err != nil {
		return "", err
	}

	return res, nil
}

func runFfprobe(args ...string) (string, error) {
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

func getWidth(input string) (int, error) {
	widthRaw, err := ffprobeOutput(input, ffprobeVideoWidth)
	if err != nil {
		return 0, err
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return 0, nil
	}
	return width, nil
}

func getDuration(input string) (float64, error) {
	durationRaw, err := ffprobeOutput(input, ffprobeDuration)
	if err != nil {
		return 0, err
	}

	durationRaw = strings.TrimSpace(durationRaw)
	if durationRaw == "" || durationRaw == "N/A" {
		return 0, nil
	}

	duration, err := strconv.ParseFloat(durationRaw, 64)
	if err != nil || duration <= 0 {
		return 0, nil
	}

	return duration, nil
}

func getAudioBitrate(input string) (int, error) {
	audioBitRateRaw, err := ffprobeOutput(input, ffprobeAudioBitrate)
	if err != nil {
		return 0, err
	}

	audioBitRateRaw = strings.TrimSpace(audioBitRateRaw)
	if audioBitRateRaw == "" || audioBitRateRaw == "N/A" {
		audioBitRateRaw, err = getRealAudioBitrate(input)
		if err != nil {
			return 0, err
		}
	}

	audioRate, err := strconv.Atoi(audioBitRateRaw)
	if err != nil || audioRate < 0 {
		return 0, nil
	}

	return audioRate, nil
}

func getRealAudioBitrate(input string) (string, error) {
	duration, err := getDuration(input)
	if err != nil {
		return "", err
	}

	outAudio, err := ffprobeOutput(input, ffprobeOutAudio)
	if err != nil {
		return "", err
	}

	var totalBits int64
	lines := strings.SplitSeq(outAudio, "\n")
	for line := range lines {
		size, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
		if err == nil {
			totalBits += size * 8
		}
	}

	return fmt.Sprintf("%d", int64(float64(totalBits)/duration)), nil
}

func getRealVideoBitrate(input string) (int, error) {
	videoBitrateRaw, err := ffprobeOutput(input, ffprobeVideoBitrate)
	if err != nil {
		return 0, err
	}

	videoBitrateRaw = strings.TrimSpace(videoBitrateRaw)
	if videoBitrateRaw != "" && videoBitrateRaw != "N/A" {
		videoBitrate, err := strconv.Atoi(videoBitrateRaw)
		if err == nil && videoBitrate > 0 {
			return videoBitrate, nil
		}
	}

	return 0, nil
}
