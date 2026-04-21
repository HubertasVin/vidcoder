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
		rec, err = getRecommendedParams(cfg.InputPath)
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
	}

	filters, err := buildVideoFilters(cfg)
	if err != nil {
		return nil, err
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}

	if cfg.VideoCodec != "" {
		args = append(args, "-c:v", cfg.VideoCodec)
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

	if cfg.AudioCodec != "" {
		args = append(args, "-c:a", cfg.AudioCodec)
	}
	if cfg.UseRecommendedAudio && cfg.AudioBitrate == "" && rec.AudioBitrate != "" {
		args = append(args, "-b:a", rec.AudioBitrate)
	}
	if cfg.AudioBitrate != "" {
		args = append(args, "-b:a", cfg.AudioBitrate)
	}
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
	filters := make([]string, 0, 2)

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

	return filters, nil
}
