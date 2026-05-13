package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func run(cfg config) error {
	var rec recommendedParams
	var err error
	if cfg.UseRecommendedVideo || cfg.UseRecommendedAudio {
		rec, err = getRecommendedParams(cfg.InputPath, cfg.CompressedSource, cfg.Encoder)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return err
	}

	args, err := buildFFmpegArgs(cfg, rec)
	if err != nil {
		return err
	}

	return runFFmpegWithRetry(args, cfg.OutputPath)
}

func runFFmpegWithRetry(args []string, outputPath string) error {
	const maxAttempts = 3

	_, outputStatErr := os.Stat(outputPath)
	outputExistedBeforeRun := outputStatErr == nil

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err == nil {
			return nil
		}
		if !isKilledProcessError(err) || attempt == maxAttempts {
			return err
		}

		if !outputExistedBeforeRun {
			if removeErr := os.Remove(outputPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("ffmpeg failed and partial output cleanup failed: %w", removeErr)
			}
		}
		fmt.Fprintf(os.Stderr, "ffmpeg was killed; retrying (%d/%d)\n", attempt+1, maxAttempts)
	}

	return nil
}

func isKilledProcessError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

func buildFFmpegArgs(cfg config, rec recommendedParams) ([]string, error) {
	args := []string{
		"-threads", "0",
		"-n",
		"-i", cfg.InputPath,
		"-map", "0:v?",
		"-map", "0:a?",
	}

	subMaps, err := subtitleStreamMaps(cfg.InputPath)
	if err != nil {
		return nil, err
	}
	args = append(args, subMaps...)

	filters, err := buildVideoFilters(cfg)
	if err != nil {
		return nil, err
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}

	if cfg.Encoder != "" {
		args = append(args, "-c:v", cfg.Encoder.codec())
	}
	if cfg.UseRecommendedVideo && rec.HasVideoPrefs {
		args = append(args, rec.VideoArgs...)
	}
	if cfg.VideoCRF != "" {
		args = append(args, "-crf", cfg.VideoCRF)
	}
	if cfg.UseRecommendedVideo && rec.VideoPreset != "" {
		args = append(args, "-preset", rec.VideoPreset)
	}
	if cfg.VideoPreset != "" {
		args = append(args, "-preset", cfg.VideoPreset)
	}

	args = append(args, "-c:s", "copy")

	if cfg.AudioCodec != "" {
		args = append(args, "-c:a", cfg.AudioCodec)
	}
	if cfg.AudioBitrate != "" {
		originalRate, probeErr := getAudioBitrate(cfg.InputPath)
		targetBitrate, convErr := strconv.Atoi(cfg.AudioBitrate)
		if convErr != nil {
			fmt.Println(convErr)
		}
		if probeErr == nil && originalRate > 0 && originalRate >= targetBitrate {
			args = append(args, "-b:a", cfg.AudioBitrate)
		}
	}
	args = append(args, "-af", "aformat=channel_layouts=5.1|7.1|stereo|mono")
	if cfg.UseRecommendedAudio && cfg.AudioCodec == "libopus" {
		args = append(args, "-application", "audio")
	}

	if cfg.ExtraFFmpeg != "" {
		args = append(args, strings.Fields(cfg.ExtraFFmpeg)...)
	}

	args = append(args, cfg.OutputPath)
	return args, nil
}

func buildVideoFilters(cfg config) ([]string, error) {
	filters := make([]string, 0, 3)

	if cfg.Resolution != "" {
		parts := strings.Split(cfg.Resolution, "x")
		filters = append(filters, "scale="+parts[0]+":"+parts[1])
	}

	if cfg.ScaleValue != "" {
		scale, err := parseScaleMultiplier(cfg.ScaleValue)
		if err != nil {
			return nil, err
		}
		scaleText := strconv.FormatFloat(scale, 'f', -1, 64)
		filters = append(filters,
			fmt.Sprintf("scale=trunc(iw*%s/2)*2:trunc(ih*%s/2)*2", scaleText, scaleText))
	}

	if cfg.Denoise {
		if cfg.CompressedSource {
			filters = append(filters, "hqdn3d=2:1.5:3:3")
		} else {
			filters = append(filters, "hqdn3d=1.5:1.5:6:6")
		}
	}

	return filters, nil
}

var matroskaSupportedSubCodecs = map[string]bool{
	"ass":              true,
	"ssa":              true,
	"srt":              true,
	"subrip":           true,
	"dvd_subtitle":     true,
	"dvdsub":           true,
	"hdmv_pgs_subtitle": true,
	"pgs":              true,
	"webvtt":           true,
}

func subtitleStreamMaps(input string) ([]string, error) {
	codecs, err := getSubtitleStreamCodecs(input)
	if err != nil {
		return []string{"-map", "0:s?"}, nil
	}

	var maps []string
	for idx, codec := range codecs {
		if matroskaSupportedSubCodecs[codec] {
			maps = append(maps, "-map", fmt.Sprintf("0:%d", idx))
		} else {
			fmt.Fprintf(os.Stderr, "Skipping unsupported subtitle stream %d (%s)\n", idx, codec)
		}
	}
	return maps, nil
}