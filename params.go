package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type recommendedParams struct {
	VideoArgs     []string
	VideoPreset   string
	AudioBitrate  string
	HasVideoPrefs bool
}

type ffprobeField int

const (
	ffprobeVideoPixelFormat ffprobeField = iota
	ffprobeAudioBitrate
	ffprobeDuration
	ffprobeVideoWidth
)

func getRecommendedParams(input string) (recommendedParams, error) {
	var rec recommendedParams

	crf, err := getCRF(input)
	if err != nil {
		return rec, err
	}
	svtParams, err := getSVTAV1Params(input)
	if err != nil {
		return rec, err
	}
	rec.VideoArgs = []string{
		"-crf", strconv.Itoa(crf),
		"-svtav1-params", svtParams,
	}

	rec.VideoPreset, err = getPreset(input)
	if err != nil {
		return rec, err
	}
	pixFmt, err := getPixFmt(input)
	if err != nil {
		return rec, err
	}
	if pixFmt != "" {
		rec.VideoArgs = append(rec.VideoArgs, "-pix_fmt", pixFmt)
	}
	rec.HasVideoPrefs = true

	audioBitRateRaw, err := ffprobeOutput(input, ffprobeAudioBitrate)
	if err != nil {
		return rec, err
	}

	audioBitRate, err := strconv.ParseInt(strings.TrimSpace(audioBitRateRaw), 10, 64)
	if err == nil {
		switch {
		case audioBitRate < 128000:
			rec.AudioBitrate = fmt.Sprintf("%dk", audioBitRate/1000)
		case audioBitRate >= 128000:
			rec.AudioBitrate = "128k"
		}
	}

	return rec, nil
}

func getPixFmt(input string) (string, error) {
	pixFmt, err := ffprobeOutput(input, ffprobeVideoPixelFormat)
	if err != nil {
		return "", err
	}

	if is10Bit(pixFmt) {
		return "yuv420p10le", nil
	}
	return "", nil
}	

func getSVTAV1Params(input string) (string, error) {
	// width, err := ffprobeOutput(input, ffprobeVideoWidth)
	// if err != nil {
	// 	return "", err
	// }
	pixFmt, err := ffprobeOutput(input, ffprobeVideoPixelFormat)
	if err != nil {
		return "", err
	}

	svtParams := "tune=0:film-grain=8"
	// if width >= "3840" {
	// 	svtParams += ":enable-dlf=0:enable-cdef=0"
	// } else {
	// 	svtParams += ":enable-fg=1"
	// }
	if is10Bit(pixFmt) {
		svtParams += ":input-depth=10"
	}

	return svtParams, nil
}

func is10Bit(pixFmt string) bool {
	return strings.Contains(pixFmt, "10le") || strings.Contains(pixFmt, "10be")
}	

func getCRF(input string) (int, error) {
	durationRaw, err := ffprobeOutput(input, ffprobeDuration)
	if err != nil {
		return 0, err
	}

	durationRaw = strings.TrimSpace(durationRaw)
	if durationRaw != "" && durationRaw != "N/A" {
		duration, err := strconv.ParseFloat(durationRaw, 64)
		if err == nil && duration > 0 {
			info, err := os.Stat(input)
			if err != nil {
				return 0, err
			}
			rate := int64(math.Round((float64(info.Size()) * 8.0) / duration))
			return rateToCRF(rate), nil
		}
	}

	widthRaw, err := ffprobeOutput(input, ffprobeVideoWidth)
	if err != nil {
		return 0, err
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return 36, nil
	}

	switch {
	case width >= 3840:
		return 24, nil
	case width >= 1920:
		return 32, nil
	case width >= 720:
		return 34, nil
	default:
		return 36, nil
	}
}

func getPreset(input string) (string, error) {
	widthRaw, err := ffprobeOutput(input, ffprobeVideoWidth)
	if err != nil {
		return "", err
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return "3", nil
	}
	if width >= 3840 {
		return "2", nil
	}
	return "3", nil
}

func rateToCRF(rate int64) int {
	switch {
	case rate >= 3_500_000:
		return 24
	case rate >= 1_800_000:
		return 32
	case rate >= 900_000:
		return 34
	default:
		return 36
	}
}

func ffprobeOutput(input string, field ffprobeField) (string, error) {
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
