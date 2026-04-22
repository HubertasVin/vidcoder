package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWidth(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920\n", nil)
	width, err := getWidth("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 1920, width)
	mockHelper.AssertExpectations(t)
}

func TestGetDuration(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeDuration).Return("123.45\n", nil)
	duration, err := getDuration("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 123.45, duration)
	mockHelper.AssertExpectations(t)
}

func TestGetAudioBitrate_Normal(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeAudioBitrate).Return("256000\n", nil)
	rate, err := getAudioBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 256000, rate)
	mockHelper.AssertExpectations(t)
}

func TestGetAudioBitrate_Fallback(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeAudioBitrate).Return("N/A", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeDuration).Return("10.0", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeOutAudio).Return("1000\n1000\n", nil)

	rate, err := getAudioBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 1600, rate) // 2000 bytes * 8 = 16000 bits / 10s = 1600 bps
	mockHelper.AssertExpectations(t)
}

func TestGetRealAudioBitrate(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeDuration).Return("5.0", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeOutAudio).Return("500\n500\n", nil)

	rate, err := getRealAudioBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, "1600", rate) // 1000 bytes * 8 = 8000 bits / 5s = 1600 bps
	mockHelper.AssertExpectations(t)
}

func TestGetRealVideoBitrate(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("5000000\n", nil)
	rate, err := getRealVideoBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 5000000, rate)
	mockHelper.AssertExpectations(t)
}
