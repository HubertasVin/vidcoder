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

func TestParseEncoder(t *testing.T) {
	tests := []struct {
		input    string
		want     encoderType
		wantErr  bool
	}{
		{"H264", encH264, false},
		{"AV1", encAV1, false},
		{"HEVC", encHEVC, false},
		{"x264", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		enc, err := parseEncoder(tt.input)
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.want, enc)
		}
	}
}

func TestEncoderCodec(t *testing.T) {
	assert.Equal(t, "libx264", encH264.codec())
	assert.Equal(t, "libsvtav1", encAV1.codec())
	assert.Equal(t, "libx265", encHEVC.codec())
}

func TestEncoderMapCRF(t *testing.T) {
	assert.Equal(t, 20, encH264.mapCRF(32))
	assert.Equal(t, 32, encAV1.mapCRF(32))
	assert.Equal(t, 22, encHEVC.mapCRF(32))
}

func TestEncoderMapPreset(t *testing.T) {
	assert.Equal(t, "slower", encH264.mapPreset("2"))
	assert.Equal(t, "slow", encH264.mapPreset("3"))
	assert.Equal(t, "medium", encH264.mapPreset("5"))
	assert.Equal(t, "slower", encHEVC.mapPreset("2"))
	assert.Equal(t, "slow", encHEVC.mapPreset("3"))
	assert.Equal(t, "medium", encHEVC.mapPreset("5"))
	assert.Equal(t, "2", encAV1.mapPreset("2"))
	assert.Equal(t, "3", encAV1.mapPreset("3"))
	assert.Equal(t, "5", encAV1.mapPreset("5"))
}

func TestIs10Bit(t *testing.T) {
	assert.True(t, is10Bit("yuv420p10le"))
	assert.True(t, is10Bit("yuv422p10be"))
	assert.False(t, is10Bit("yuv420p"))
	assert.False(t, is10Bit(""))
}

func TestResAndRateToCRF(t *testing.T) {
	assert.Equal(t, 30, resAndRateToCRF(3840, 6_000_000))
	assert.Equal(t, 30, resAndRateToCRF(3840, 5_000_000))
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
	mockFileInfo.On("Size").Return(1250000)
	mockHelper.On("osStat", "input.mkv").Return(mockFileInfo, nil)

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeAudioBitrate).Return("128000", nil)

	rate, err := getVideoBitrate("input.mkv")
	assert.NoError(t, err)
	assert.Equal(t, 872000, rate)
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

func TestRecommendEncoderParamsAV1(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)
	params, err := recommendEncoderParams("input.mkv", false, encAV1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-svtav1-params", "tune=0:enable-dlf=0:enable-cdef=0:input-depth=10"}, params)
	mockHelper.AssertExpectations(t)
}

func TestRecommendEncoderParamsAV1Compressed(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)
	params, err := recommendEncoderParams("input.mkv", true, encAV1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-svtav1-params", "tune=2:enable-variance-boost=1:input-depth=10"}, params)
	mockHelper.AssertExpectations(t)
}

func TestRecommendEncoderParamsH264(t *testing.T) {
	params, err := recommendEncoderParams("", false, encH264)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-tune", "film"}, params)
}

func TestRecommendEncoderParamsH264Compressed(t *testing.T) {
	params, err := recommendEncoderParams("", true, encH264)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-tune", "animation"}, params)
}

func TestRecommendEncoderParamsHEVC(t *testing.T) {
	params, err := recommendEncoderParams("", false, encHEVC)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-tune", "grain"}, params)
}

func TestRecommendEncoderParamsHEVCCompressed(t *testing.T) {
	params, err := recommendEncoderParams("", true, encHEVC)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-tune", "animation"}, params)
}

func TestGetRecommendedParamsAV1(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil).Times(2)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil).Times(2)

	rec, err := getRecommendedParams("input.mkv", false, encAV1)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "3", rec.VideoPreset)
	assert.Equal(t, []string{"-crf", "32", "-svtav1-params", "tune=0:enable-dlf=0:enable-cdef=0:input-depth=10", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParamsAV1Compressed(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil).Times(2)

	rec, err := getRecommendedParams("input.mkv", true, encAV1)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "3", rec.VideoPreset)
	assert.Equal(t, []string{
		"-crf", "33",
		"-svtav1-params", "tune=2:enable-variance-boost=1:input-depth=10",
		"-pix_fmt", "yuv420p10le",
	}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParamsH264(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil).Times(2)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)

	rec, err := getRecommendedParams("input.mkv", false, encH264)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "slow", rec.VideoPreset)
	assert.Equal(t, []string{"-crf", "20", "-tune", "film", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParamsH264Compressed(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)

	rec, err := getRecommendedParams("input.mkv", true, encH264)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "slow", rec.VideoPreset)
	assert.Equal(t, []string{"-crf", "21", "-tune", "animation", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParamsHEVC(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil).Times(2)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)

	rec, err := getRecommendedParams("input.mkv", false, encHEVC)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "slow", rec.VideoPreset)
	assert.Equal(t, []string{"-crf", "22", "-tune", "grain", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}

func TestGetRecommendedParamsHEVCCompressed(t *testing.T) {
	mockHelper := new(MockHelper)
	originalFfprobeOutput := ffprobeOutput
	ffprobeOutput = mockHelper.ffprobeOutput
	defer func() { ffprobeOutput = originalFfprobeOutput }()

	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoBitrate).Return("2000000", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoWidth).Return("1920", nil)
	mockHelper.On("ffprobeOutput", "input.mkv", ffprobeVideoPixelFormat).Return("yuv420p10le", nil)

	rec, err := getRecommendedParams("input.mkv", true, encHEVC)
	assert.NoError(t, err)
	assert.True(t, rec.HasVideoPrefs)
	assert.Equal(t, "slow", rec.VideoPreset)
	assert.Equal(t, []string{"-crf", "23", "-tune", "animation", "-pix_fmt", "yuv420p10le"}, rec.VideoArgs)
	mockHelper.AssertExpectations(t)
}