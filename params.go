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
	AudioBitrate  string
	HasVideoPrefs bool
}

func getRecommendedParams(input string) (recommendedParams, error) {
	var rec recommendedParams

	pixFmt, err := ffprobeOutput(
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=pix_fmt",
		"-of", "csv=p=0",
		input,
	)
	if err != nil {
		return rec, err
	}

	svtParams := "tune=0:enable-dlf=0:enable-cdef=0"
	pixOut := ""
	if strings.Contains(pixFmt, "10le") || strings.Contains(pixFmt, "10be") {
		svtParams = "tune=0:input-depth=10:enable-dlf=0:enable-cdef=0"
		pixOut = "yuv420p10le"
	}

	crf, err := chooseRecommendedCRF(input)
	if err != nil {
		return rec, err
	}

	rec.VideoArgs = []string{
		"-crf", strconv.Itoa(crf),
		"-svtav1-params", svtParams,
	}
	if pixOut != "" {
		rec.VideoArgs = append(rec.VideoArgs, "-pix_fmt", pixOut)
	}
	rec.HasVideoPrefs = true

	audioBitRateRaw, err := ffprobeOutput(
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=bit_rate",
		"-of", "csv=p=0",
		input,
	)
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

func chooseRecommendedCRF(input string) (int, error) {
	durationRaw, err := ffprobeOutput(
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		input,
	)
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

	widthRaw, err := ffprobeOutput(
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width",
		"-of", "csv=p=0",
		input,
	)
	if err != nil {
		return 0, err
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return 36, nil
	}

	switch {
	case width >= 3840:
		return 34, nil
	case width >= 1920:
		return 32, nil
	case width >= 720:
		return 34, nil
	default:
		return 36, nil
	}
}

func rateToCRF(rate int64) int {
	switch {
	case rate >= 3_500_000:
		return 34
	case rate >= 1_800_000:
		return 32
	case rate >= 900_000:
		return 34
	default:
		return 36
	}
}

func ffprobeOutput(args ...string) (string, error) {
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
