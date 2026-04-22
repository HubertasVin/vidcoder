package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var osStat = os.Stat

type recommendedParams struct {
	VideoArgs     []string
	VideoPreset   string
	AudioBitrate  string
	HasVideoPrefs bool
}

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
	pixFmt, err := ffprobeOutput(input, ffprobeVideoPixelFormat)
	if err != nil {
		return "", err
	}

	svtParams := "tune=0:enable-dlf=0:enable-cdef=0"
	if is10Bit(pixFmt) {
		svtParams += ":input-depth=10"
	}

	return svtParams, nil
}

func is10Bit(pixFmt string) bool {
	return strings.Contains(pixFmt, "10le") || strings.Contains(pixFmt, "10be")
}

func getCRF(input string) (int, error) {
	width, err := getWidth(input)
	if err != nil {
		return 0, err
	}

	rate, err := getVideoRate(input)
	if err != nil {
		return 0, err
	}

	return resAndRateToCRF(width, rate), nil
}

func getVideoRate(input string) (int, error) {

	if videoRate, err := getFfprobeVideoRate(input); err == nil {
		if videoRate > 0 {
			return videoRate, nil
		}
	} else {
		return 0, err
	}

	duration, err := getDuration(input)
	if err != nil {
		return 0, err
	}

	info, err := osStat(input)
	if err != nil {
		return 0, err
	}
	totalRate := int((float64(info.Size()) * 8.0) / duration)

	audioRate, err := getAudioRate(input)
	if err != nil {
		return 0, err
	}

	videoRate := totalRate - audioRate
	if videoRate < 0 {
		return 0, nil
	}

	return videoRate, nil
}

func getAudioRate(input string) (int, error) {
	audioBitRateRaw, err := ffprobeOutput(input, ffprobeAudioBitrate)
	if err != nil {
		return 0, err
	}

	audioBitRateRaw = strings.TrimSpace(audioBitRateRaw)
	if audioBitRateRaw != "" && audioBitRateRaw != "N/A" {
		audioBitRate, err := strconv.Atoi(audioBitRateRaw)
		if err == nil && audioBitRate >= 0 {
			return audioBitRate, nil
		}
	}

	return 0, nil
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

func resAndRateToCRF(res, rate int) int {
	switch {
	case res >= 3840:
		if rate >= 6_000_000 {
			return 26
		} else {
			return 27
		}
	case res >= 1920:
		if rate >= 2_000_000 {
			return 32
		} else {
			return 34
		}
	case res >= 720:
		if rate >= 1_000_000 {
			return 34
		} else {
			return 36
		}
	default:
		return 36
	}
}

func getFfprobeVideoRate(input string) (int, error) {
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
