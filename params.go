package main

import (
	"os"
	"strconv"
	"strings"
)

var osStat = os.Stat

type recommendedParams struct {
	VideoArgs     []string
	VideoPreset   string
	HasVideoPrefs bool
}

func getRecommendedParams(input string) (recommendedParams, error) {
	var rec recommendedParams

	crf, err := recommendCRF(input)
	if err != nil {
		return rec, err
	}
	svtParams, err := recommendSVTAV1Params(input)
	if err != nil {
		return rec, err
	}
	rec.VideoArgs = []string{
		"-crf", strconv.Itoa(crf),
		"-svtav1-params", svtParams,
	}

	rec.VideoPreset, err = recommendPreset(input)
	if err != nil {
		return rec, err
	}
	pixFmt, err := recommendPixFmt(input)
	if err != nil {
		return rec, err
	}
	if pixFmt != "" {
		rec.VideoArgs = append(rec.VideoArgs, "-pix_fmt", pixFmt)
	}
	rec.HasVideoPrefs = true

	return rec, nil
}

func recommendPixFmt(input string) (string, error) {
	pixFmt, err := ffprobeOutput(input, ffprobeVideoPixelFormat)
	if err != nil {
		return "", err
	}

	if is10Bit(pixFmt) {
		return "yuv420p10le", nil
	}
	return "", nil
}

func recommendSVTAV1Params(input string) (string, error) {
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

func recommendCRF(input string) (int, error) {
	width, err := getWidth(input)
	if err != nil {
		return 0, err
	}

	rate, err := getVideoBitrate(input)
	if err != nil {
		return 0, err
	}

	return resAndRateToCRF(width, rate), nil
}

func recommendPreset(input string) (string, error) {
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

func getVideoBitrate(input string) (int, error) {
	if videoRate, err := getRealVideoBitrate(input); err == nil {
		if videoRate > 0 {
			return videoRate, nil
		}
	} else {
		return 0, err
	}

	totalRate, err := getTotalBitrate(input)
	if err != nil {
		return 0, err
	}

	audioRate, err := getAudioBitrate(input)
	if err != nil {
		return 0, err
	}

	videoRate := totalRate - audioRate
	if videoRate < 0 {
		return 0, nil
	}

	return videoRate, nil
}

func getTotalBitrate(input string) (int, error) {
	duration, err := getDuration(input)
	if err != nil {
		return 0, err
	}

	info, err := osStat(input)
	if err != nil {
		return 0, err
	}
	totalRate := int((float64(info.Size()) * 8.0) / duration)

	return totalRate, nil
}

func resAndRateToCRF(res, rate int) int {
	switch {
	case res >= 3840:
		if rate >= 6_000_000 {
			return 26
		} else {
			return 28
		}
	case res >= 1920:
		if rate >= 2_000_000 {
			return 32
		} else {
			return 35
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

func is10Bit(pixFmt string) bool {
	return strings.Contains(pixFmt, "10le") || strings.Contains(pixFmt, "10be")
}
