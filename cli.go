package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
)

var (
	errHelp         = errors.New("help requested")
	resolutionRegex = regexp.MustCompile(`^[0-9]+x[0-9]+$`)
)

type config struct {
	InputPath  string
	OutputPath string

	UseRecommendedAll   bool
	UseRecommendedVideo bool
	UseRecommendedAudio bool

	ShowVersion bool

	VideoCodec string
	AudioCodec string

	VideoCRF     string
	VideoPreset  string
	AudioBitrate string

	Resolution string
	ScaleValue string

	ExtraFFmpeg string

	videoCodecSet bool
	audioCodecSet bool
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: vidcoder [options] [input] [output]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	var cfg config
	fs, _ := newFlagSet(&cfg, w)
	fs.PrintDefaults()
}

func parseArgs(args []string) (config, error) {
	var cfg config

	fs, helpRequested := newFlagSet(&cfg, io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) || *helpRequested {
			return cfg, errHelp
		}
		return cfg, err
	}
	if *helpRequested {
		return cfg, errHelp
	}

	cfg.videoCodecSet = fs.Changed("video-codec")
	cfg.audioCodecSet = fs.Changed("audio-codec")

	if cfg.ShowVersion {
		if len(fs.Args()) > 0 {
			return cfg, errors.New("--version does not accept positional arguments")
		}
		return cfg, nil
	}

	for _, positional := range fs.Args() {
		if err := setPositional(&cfg, positional); err != nil {
			return cfg, err
		}
	}

	if cfg.InputPath == "" {
		return cfg, errors.New("input file is required. Use -i/--input or positional input")
	}
	if _, err := os.Stat(cfg.InputPath); err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("input file does not exist: %s", cfg.InputPath)
		}
		return cfg, err
	}

	if cfg.Resolution != "" && !resolutionRegex.MatchString(cfg.Resolution) {
		return cfg, fmt.Errorf("resolution must be in WxH format (e.g. 1280x720): %s", cfg.Resolution)
	}
	if cfg.Resolution != "" && cfg.ScaleValue != "" {
		return cfg, errors.New("use either --resolution or --scale, not both")
	}
	if cfg.ScaleValue != "" {
		if _, err := parseScaleMultiplier(cfg.ScaleValue); err != nil {
			return cfg, err
		}
	}

	if cfg.UseRecommendedAll {
		cfg.UseRecommendedVideo = true
		cfg.UseRecommendedAudio = true
		if !cfg.videoCodecSet {
			cfg.VideoCodec = "libsvtav1"
		}
		if !cfg.audioCodecSet {
			cfg.AudioCodec = "libopus"
		}
	}

	if cfg.OutputPath == "" {
		cfg.OutputPath = defaultOutputPath(cfg.InputPath)
	}

	return cfg, nil
}

func defaultOutputPath(input string) string {
	out := "AV1/" + strings.TrimPrefix(input, "./")
	ext := filepath.Ext(out)
	if ext != "" {
		out = strings.TrimSuffix(out, ext)
	}
	return out + ".mkv"
}

func newFlagSet(cfg *config, output io.Writer) (*pflag.FlagSet, *bool) {
	fs := pflag.NewFlagSet("vidcoder", pflag.ContinueOnError)
	fs.SetOutput(output)

	helpRequested := false
	fs.BoolVarP(&helpRequested, "help", "h", false, "show help")
	fs.BoolVarP(&cfg.ShowVersion, "version", "v", false, "show version")
	fs.StringVarP(&cfg.InputPath, "input", "i", "", "input file path")
	fs.StringVarP(&cfg.OutputPath, "output", "o", "", "output file path")
	fs.BoolVar(&cfg.UseRecommendedAll, "recommended", false, "recommended settings")
	fs.BoolVar(&cfg.UseRecommendedVideo, "recommended-video", false, "recommended video settings")
	fs.BoolVar(&cfg.UseRecommendedAudio, "recommended-audio", false, "recommended audio settings")
	fs.StringVar(&cfg.VideoCodec, "video-codec", "", "video codec")
	fs.StringVar(&cfg.AudioCodec, "audio-codec", "", "audio codec")
	fs.StringVar(&cfg.VideoCRF, "crf", "", "video quality CRF value")
	fs.StringVar(&cfg.VideoPreset, "preset", "", "video encoder preset")
	fs.StringVar(&cfg.AudioBitrate, "audio-bitrate", "", "audio bitrate")
	fs.StringVar(&cfg.Resolution, "resolution", "", "output resolution (WxH)")
	fs.StringVarP(&cfg.ScaleValue, "scale", "s", "", "scale factor (*2, /2, x1.5, 1.5)")
	fs.StringVarP(&cfg.ExtraFFmpeg, "ffmpeg", "f", "", "additional ffmpeg arguments")

	return fs, &helpRequested
}

func setPositional(cfg *config, value string) error {
	if cfg.InputPath == "" {
		cfg.InputPath = value
		return nil
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = value
		return nil
	}
	return fmt.Errorf("unexpected argument: %s", value)
}
