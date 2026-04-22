package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockHelper struct {
	mock.Mock
}

func (m *MockHelper) ffprobeOutput(input string, field ffprobeField) (string, error) {
	args := m.Called(input, field)
	return args.String(0), args.Error(1)
}

func (m *MockHelper) osStat(name string) (os.FileInfo, error) {
	args := m.Called(name)
	if info := args.Get(0); info != nil {
		return info.(os.FileInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockFileInfo struct {
	mock.Mock
}

func (m *MockFileInfo) Name() string       { return "mock" }
func (m *MockFileInfo) Size() int64        { args := m.Called(); return int64(args.Int(0)) }
func (m *MockFileInfo) Mode() os.FileMode  { return 0 }
func (m *MockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *MockFileInfo) IsDir() bool        { return false }
func (m *MockFileInfo) Sys() any           { return nil }

func TestIs10Bit(t *testing.T) {
	assert.True(t, is10Bit("yuv420p10le"))
	assert.True(t, is10Bit("yuv422p10be"))
	assert.False(t, is10Bit("yuv420p"))
	assert.False(t, is10Bit(""))
}

func TestResAndRateToCRF(t *testing.T) {
	assert.Equal(t, 26, resAndRateToCRF(3840, 6_000_000))
	assert.Equal(t, 28, resAndRateToCRF(3840, 5_000_000))
	assert.Equal(t, 32, resAndRateToCRF(1920, 2_000_000))
	assert.Equal(t, 35, resAndRateToCRF(1920, 1_000_000))
	assert.Equal(t, 34, resAndRateToCRF(1280, 1_000_000))
	assert.Equal(t, 36, resAndRateToCRF(1280, 500_000))
	assert.Equal(t, 36, resAndRateToCRF(640, 500_000))
}

func TestGetVideoRateFromFfprobe(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000\n", nil)
	rate, err := getVideoBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 2000000, rate)
	mockHelper.AssertExpectations(t)
}

func TestGetVideoRateFromSize(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	originalOsStat := osStat
	ffprobeOutput = mockHelper.ffprobeOutput
	osStat = mockHelper.osStat
	defer func() {
		ffprobeOutput = originalFfprobeOutput
		osStat = originalOsStat
	}()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("N/A", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeDuration).Return("10.0", nil)

	mockFileInfo := new(MockFileInfo)
	mockFileInfo.On("Size").Return(1250000) // 10,000,000 bits / 10s = 1,000,000 bit/s total
	mockHelper.On("osStat", "input.mkv").Return(mockFileInfo, nil)

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeAudioBitrate).Return("128000", nil)

	rate, err := getVideoBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 872000, rate) // 1000000 - 128000
	mockHelper.AssertExpectations(t)
	mockFileInfo.AssertExpectations(t)
}

func TestGetPixFmt(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)
	pixFmt, err := recommendPixFmt("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, "yuv420p10le", pixFmt)
	mockHelper.AssertExpectations(t)
}

func TestGetSVTAV1Params(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)
	params, err := recommendSVTAV1Params("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, "tune=0:enable-dlf=0:enable-cdef=0:input-depth=10", params)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParams(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil).Times(2)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil).Times(2)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeAudioBitrate).Return("256000", nil)

	rec, err := getRecommendedParams("input.mkv")
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "3", rec.VideoPreset)
	assert.Equal(t, "128k", rec.AudioBitrate)
	assert.Equal(t, []string{"-crf", "32", "-svtav1-params", "tune=0:enable-dlf=0:enable-cdef=0:input-depth=10", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}
