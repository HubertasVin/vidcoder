package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var osStat = os.Stat

type encoderType string

const (
	encH264 encoderType = "H264"
	encAV1  encoderType = "AV1"
	encHEVC encoderType = "HEVC"
)

func parseEncoder(s string) (encoderType, error) {
	switch s {
	case "H264":
		return encH264, nil
	case "AV1":
		return encAV1, nil
	case "HEVC":
		return encHEVC, nil
	default:
		return "", fmt.Errorf("invalid encoder: %s (valid: H264, AV1, HEVC)", s)
	}
}

func (e encoderType) codec() string {
	switch e {
	case encH264:
		return "libx264"
	case encAV1:
		return "libsvtav1"
	case encHEVC:
		return "libx265"
	}
	return ""
}

func (e encoderType) mapCRF(av1CRF int) int {
	switch e {
	case encH264:
		return av1CRF - 12
	case encAV1:
		return av1CRF
	case encHEVC:
		return av1CRF - 10
	}
	return av1CRF
}

func (e encoderType) mapPreset(av1Preset string) string {
	switch e {
	case encH264, encHEVC:
		switch av1Preset {
		case "2":
			return "slower"
		case "3":
			return "slow"
		case "5":
			return "medium"
		default:
			return "slow"
		}
	default:
		return av1Preset
	}
}

type recommendedParams struct {
	VideoArgs     []string
	VideoPreset   string
	HasVideoPrefs bool
}

func getRecommendedParams(input string, compressedSource bool, enc encoderType) (recommendedParams, error) {
	var rec recommendedParams

	crf, err := recommendCRF(input)
	if err != nil {
		return rec, err
	}
	if compressedSource {
		crf += 2
	}
	mappedCRF := enc.mapCRF(crf)

	encParams, err := recommendEncoderParams(input, compressedSource, enc)
	if err != nil {
		return rec, err
	}
	rec.VideoArgs = append([]string{"-crf", strconv.Itoa(mappedCRF)}, encParams...)

	rec.VideoPreset, err = recommendPreset(input, enc)
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

func recommendEncoderParams(input string, compressedSource bool, enc encoderType) ([]string, error) {
	switch enc {
	case encAV1:
		pixFmt, err := ffprobeOutput(input, ffprobeVideoPixelFormat)
		if err != nil {
			return nil, err
		}
		return recommendSVTAV1Args(pixFmt, compressedSource), nil
	case encH264:
		tune := "film"
		if compressedSource {
			tune = "animation"
		}
		return []string{"-tune", tune}, nil
	case encHEVC:
		tune := "grain"
		if compressedSource {
			tune = "animation"
		}
		return []string{"-tune", tune}, nil
	}
	return nil, nil
}

func recommendSVTAV1Args(pixFmt string, compressedSource bool) []string {
	if compressedSource {
		svtParams := "tune=2:enable-variance-boost=1"
		if is10Bit(pixFmt) {
			svtParams += ":input-depth=10"
		}
		return []string{"-svtav1-params", svtParams}
	}

	svtParams := "tune=0:enable-dlf=0:enable-cdef=0"
	if is10Bit(pixFmt) {
		svtParams += ":input-depth=10"
	}
	return []string{"-svtav1-params", svtParams}
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

func recommendPreset(input string, enc encoderType) (string, error) {
	widthRaw, err := ffprobeOutput(input, ffprobeVideoWidth)
	if err != nil {
		return "", err
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return enc.mapPreset("3"), nil
	}
	var av1Preset string
	if width >= 3840 {
		av1Preset = "2"
	} else {
		av1Preset = "3"
	}
	return enc.mapPreset(av1Preset), nil
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
		if rate >= 5_000_000 {
			return 30
		} else {
			return 32
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
