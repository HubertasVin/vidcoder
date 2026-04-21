# vidcoder

`vidcoder` is a lightweight CLI wrapper around `ffmpeg` for quick video transcoding, with built-in AV1-oriented recommended settings.

# Installation

Install the latest release binary via Go:

```bash
go install github.com/HubertasVin/vidcoder@latest
```

# Usage

```bash
vidcoder [options] [input] [output]
```

Use `--recommended` flag to apply the recommended settings with AV1 encoding:

```bash
vidcoder --recommended input.mp4 output.mkv
```
