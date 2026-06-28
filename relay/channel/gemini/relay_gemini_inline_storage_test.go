package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOffloadGeminiInlineImagesDisabledLeavesResponseUnchanged(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, system_setting.GeneratedImageStorageSettings{
		Enabled: false,
	})
	defer restore()

	c := newGeminiStorageTestContext()
	response := geminiStorageResponse(dto.GeminiPart{
		InlineData: &dto.GeminiInlineData{
			MimeType: "image/png",
			Data:     testGeneratedImageBase64(1024*1024 + 1),
		},
	})

	called := false
	uploadGeneratedImage = func(context.Context, service.GeneratedImageUploadMeta, string) (string, string, int64, error) {
		called = true
		return "", "", 0, nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-disabled"}, &response)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, called)
	require.NotNil(t, response.Candidates[0].Content.Parts[0].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[0].FileData)
}

func TestOffloadGeminiInlineImagesSkipsBelowThresholdAndNonImages(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFallbackInline))
	defer restore()

	c := newGeminiStorageTestContext()
	response := geminiStorageResponse(
		dto.GeminiPart{
			InlineData: &dto.GeminiInlineData{
				MimeType: "image/png",
				Data:     testGeneratedImageBase64(512),
			},
		},
		dto.GeminiPart{
			InlineData: &dto.GeminiInlineData{
				MimeType: "audio/wav",
				Data:     testGeneratedImageBase64(1024*1024 + 1),
			},
		},
		dto.GeminiPart{
			InlineData: &dto.GeminiInlineData{
				MimeType: "",
				Data:     testGeneratedImageBase64(1024*1024 + 1),
			},
		},
	)

	var uploadCount int
	uploadGeneratedImage = func(context.Context, service.GeneratedImageUploadMeta, string) (string, string, int64, error) {
		uploadCount++
		return "", "", 0, nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-skip"}, &response)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 0, uploadCount)
	for _, part := range response.Candidates[0].Content.Parts {
		assert.NotNil(t, part.InlineData)
		assert.Nil(t, part.FileData)
	}
}

func TestOffloadGeminiInlineImagesUploadsAndReplacesAllEligibleParts(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFallbackInline))
	defer restore()

	c := newGeminiStorageTestContext()
	response := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Parts: []dto.GeminiPart{
						{Text: "hello"},
						{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: testGeneratedImageBase64(1024*1024 + 1)}},
					},
				},
			},
			{
				Content: dto.GeminiChatContent{
					Parts: []dto.GeminiPart{
						{InlineData: &dto.GeminiInlineData{MimeType: "image/webp", Data: testGeneratedImageBase64(1024*1024 + 2)}},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{TotalTokenCount: 9},
	}

	var mu sync.Mutex
	var metas []service.GeneratedImageUploadMeta
	uploadGeneratedImage = func(_ context.Context, meta service.GeneratedImageUploadMeta, data string) (string, string, int64, error) {
		mu.Lock()
		metas = append(metas, meta)
		mu.Unlock()
		key := fmt.Sprintf("gemini/generated/2026/06/28/%s-%d-%d.bin", meta.RequestID, meta.CandidateIndex, meta.PartIndex)
		return "https://cdn.example.com/" + key, key, service.EstimateBase64DecodedBytes(data), nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-upload"}, &response)

	require.NoError(t, err)
	assert.True(t, changed)
	require.Len(t, metas, 2)
	assert.ElementsMatch(t, []service.GeneratedImageUploadMeta{
		{RequestID: "req-upload", CandidateIndex: 0, PartIndex: 1, MimeType: "image/png"},
		{RequestID: "req-upload", CandidateIndex: 1, PartIndex: 0, MimeType: "image/webp"},
	}, metas)
	assert.NotNil(t, response.Candidates[0].Content.Parts[1].FileData)
	assert.Nil(t, response.Candidates[0].Content.Parts[1].InlineData)
	assert.Equal(t, "image/png", response.Candidates[0].Content.Parts[1].FileData.MimeType)
	assert.Contains(t, response.Candidates[0].Content.Parts[1].FileData.FileUri, "req-upload-0-1.bin")
	assert.NotNil(t, response.Candidates[1].Content.Parts[0].FileData)
	assert.Nil(t, response.Candidates[1].Content.Parts[0].InlineData)
	assert.Equal(t, 9, response.UsageMetadata.TotalTokenCount)
}

func TestOffloadGeminiInlineImagesFallbackKeepsFailedInlineData(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFallbackInline))
	defer restore()

	c := newGeminiStorageTestContext()
	response := geminiStorageResponse(
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: testGeneratedImageBase64(1024*1024 + 1)}},
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: testGeneratedImageBase64(1024*1024 + 2)}},
	)

	uploadGeneratedImage = func(_ context.Context, meta service.GeneratedImageUploadMeta, data string) (string, string, int64, error) {
		if meta.PartIndex == 1 {
			return "", "", 0, errors.New("fake upload failed")
		}
		return "https://cdn.example.com/ok.png", "gemini/generated/ok.png", service.EstimateBase64DecodedBytes(data), nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-fallback"}, &response)

	require.NoError(t, err)
	assert.True(t, changed)
	assert.Nil(t, response.Candidates[0].Content.Parts[0].InlineData)
	assert.NotNil(t, response.Candidates[0].Content.Parts[0].FileData)
	assert.NotNil(t, response.Candidates[0].Content.Parts[1].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[1].FileData)
}

func TestOffloadGeminiInlineImagesFailRequestDoesNotMutateOnUploadFailure(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFailRequest))
	defer restore()

	c := newGeminiStorageTestContext()
	response := geminiStorageResponse(
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: testGeneratedImageBase64(1024*1024 + 1)}},
		dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: testGeneratedImageBase64(1024*1024 + 2)}},
	)

	uploadGeneratedImage = func(_ context.Context, meta service.GeneratedImageUploadMeta, data string) (string, string, int64, error) {
		if meta.PartIndex == 1 {
			return "", "", 0, errors.New("fake upload failed")
		}
		return "https://cdn.example.com/ok.png", "gemini/generated/ok.png", service.EstimateBase64DecodedBytes(data), nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-strict"}, &response)

	require.Error(t, err)
	assert.False(t, changed)
	assert.NotNil(t, response.Candidates[0].Content.Parts[0].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[0].FileData)
	assert.NotNil(t, response.Candidates[0].Content.Parts[1].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[1].FileData)
}

func TestOffloadGeminiInlineImagesMaxImageLimitUsesFailurePolicy(t *testing.T) {
	cfg := generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFallbackInline)
	cfg.MaxImageMB = 1
	restore := configureGeneratedImageStorageForTest(t, cfg)
	defer restore()

	c := newGeminiStorageTestContext()
	response := geminiStorageResponse(dto.GeminiPart{
		InlineData: &dto.GeminiInlineData{
			MimeType: "image/png",
			Data:     testGeneratedImageBase64(1024*1024 + 1),
		},
	})

	var uploadCount int
	uploadGeneratedImage = func(context.Context, service.GeneratedImageUploadMeta, string) (string, string, int64, error) {
		uploadCount++
		return "", "", 0, nil
	}

	changed, err := offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-limit"}, &response)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 0, uploadCount)
	assert.NotNil(t, response.Candidates[0].Content.Parts[0].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[0].FileData)

	cfg.FailurePolicy = system_setting.GeneratedImageStorageFailurePolicyFailRequest
	*system_setting.GetGeneratedImageStorageSettings() = cfg

	changed, err = offloadGeminiInlineImages(c, &relaycommon.RelayInfo{RequestId: "req-limit"}, &response)

	require.Error(t, err)
	assert.False(t, changed)
	assert.Equal(t, 0, uploadCount)
}

func TestGeminiTextGenerationHandlerOffloadsInlineImageAndUpdatesContentLength(t *testing.T) {
	restore := configureGeneratedImageStorageForTest(t, generatedImageStorageTestSettings(system_setting.GeneratedImageStorageFailurePolicyFallbackInline))
	defer restore()

	uploadGeneratedImage = func(_ context.Context, meta service.GeneratedImageUploadMeta, data string) (string, string, int64, error) {
		key := fmt.Sprintf("gemini/generated/2026/06/28/%s-%d-%d.png", meta.RequestID, meta.CandidateIndex, meta.PartIndex)
		return "https://cdn.example.com/" + key, key, service.EstimateBase64DecodedBytes(data), nil
	}

	response := geminiStorageResponse(dto.GeminiPart{
		InlineData: &dto.GeminiInlineData{
			MimeType: "image/png",
			Data:     testGeneratedImageBase64(1024*1024 + 1),
		},
	})
	body, err := common.Marshal(response)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(common.RequestIdKey, "req-handler")
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:generateContent", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := GeminiTextGenerationHandler(c, &relaycommon.RelayInfo{RequestId: "req-handler"}, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "inlineData")
	assert.Contains(t, recorder.Body.String(), "fileData")
	assert.Contains(t, recorder.Body.String(), "https://cdn.example.com/gemini/generated/2026/06/28/req-handler-0-0.png")
	assert.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))
}

func configureGeneratedImageStorageForTest(t *testing.T, cfg system_setting.GeneratedImageStorageSettings) func() {
	t.Helper()

	originalSettings := *system_setting.GetGeneratedImageStorageSettings()
	originalUploader := uploadGeneratedImage
	*system_setting.GetGeneratedImageStorageSettings() = cfg

	return func() {
		*system_setting.GetGeneratedImageStorageSettings() = originalSettings
		uploadGeneratedImage = originalUploader
	}
}

func generatedImageStorageTestSettings(failurePolicy string) system_setting.GeneratedImageStorageSettings {
	return system_setting.GeneratedImageStorageSettings{
		Enabled:              true,
		Provider:             system_setting.GeneratedImageStorageProviderAliyunOSS,
		CredentialMode:       system_setting.GeneratedImageStorageCredentialModeEnv,
		PresignEnabled:       true,
		PresignTTLSeconds:    3600,
		ObjectPrefix:         "gemini/generated",
		ThresholdMB:          1,
		MaxImageMB:           64,
		MaxTotalMB:           128,
		MaxUploadConcurrency: 2,
		UploadTimeoutSeconds: 60,
		FailurePolicy:        failurePolicy,
	}
}

func newGeminiStorageTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(common.RequestIdKey, "req-context")
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func geminiStorageResponse(parts ...dto.GeminiPart) dto.GeminiChatResponse {
	return dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Parts: parts,
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     3,
			CandidatesTokenCount: 4,
			TotalTokenCount:      7,
		},
	}
}

func testGeneratedImageBase64(size int) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, size))
}
