package gemini

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func GeminiTextGenerationHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	readStartedAt := time.Now()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		relaycommon.GeminiImageTrace(c, info, "upstream_body_read_failed", readStartedAt, "error", err.Error())
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	relaycommon.GeminiImageTrace(c, info, "upstream_body_read_done", readStartedAt,
		"body_bytes", len(responseBody),
	)

	parseStartedAt := time.Now()
	var geminiResponse dto.GeminiChatResponse
	err = common.Unmarshal(responseBody, &geminiResponse)
	if err != nil {
		relaycommon.GeminiImageTrace(c, info, "upstream_response_parse_failed", parseStartedAt, "error", err.Error())
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	inlineImageCount, inlineBase64Chars, textPreview := geminiResponseDebugSummary(&geminiResponse)
	relaycommon.GeminiImageTrace(c, info, "upstream_response_parse_done", parseStartedAt,
		"candidates", len(geminiResponse.Candidates),
		"inline_images", inlineImageCount,
		"inline_base64_chars", inlineBase64Chars,
	)
	logger.LogDebug(c, "Gemini native response summary: body_bytes=%d candidates=%d inline_images=%d inline_base64_chars=%d text_preview=%q", len(responseBody), len(geminiResponse.Candidates), inlineImageCount, inlineBase64Chars, textPreview)

	if len(geminiResponse.Candidates) == 0 {
		usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
		if geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			relaycommon.GeminiImageTrace(c, info, "upstream_generation_empty", time.Time{},
				"block_reason", *geminiResponse.PromptFeedback.BlockReason,
			)
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
			return &usage, types.NewOpenAIError(
				errors.New("request blocked by Gemini API: "+*geminiResponse.PromptFeedback.BlockReason),
				types.ErrorCodePromptBlocked,
				http.StatusBadRequest,
			)
		}

		relaycommon.GeminiImageTrace(c, info, "upstream_generation_empty", time.Time{})
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "gemini_empty_candidates")
		return &usage, types.NewOpenAIError(
			errors.New("empty response from Gemini API"),
			types.ErrorCodeEmptyResponse,
			http.StatusInternalServerError,
		)
	} else {
		relaycommon.GeminiImageTrace(c, info, "upstream_generation_success", time.Time{},
			"candidates", len(geminiResponse.Candidates),
			"inline_images", inlineImageCount,
			"inline_base64_chars", inlineBase64Chars,
		)
	}

	offloadStartedAt := time.Now()
	changed, err := offloadGeminiInlineImages(c, info, &geminiResponse)
	if err != nil {
		relaycommon.GeminiImageTrace(c, info, "inline_image_offload_failed", offloadStartedAt, "error", err.Error())
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	relaycommon.GeminiImageTrace(c, info, "inline_image_offload_done", offloadStartedAt, "changed", changed)
	if changed {
		marshalStartedAt := time.Now()
		responseBody, err = common.Marshal(geminiResponse)
		if err != nil {
			relaycommon.GeminiImageTrace(c, info, "response_rewrite_failed", marshalStartedAt, "error", err.Error())
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		relaycommon.GeminiImageTrace(c, info, "response_rewrite_done", marshalStartedAt,
			"body_bytes", len(responseBody),
		)
	}

	usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())

	writeStartedAt := time.Now()
	service.IOCopyBytesGracefully(c, resp, responseBody)
	relaycommon.GeminiImageTrace(c, info, "downstream_body_write_done", writeStartedAt,
		"body_bytes", len(responseBody),
	)

	return &usage, nil
}

func NativeGeminiEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	logger.LogDebug(c, "Gemini native embedding response body: %s", responseBody)

	usage := service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())

	if info.IsGeminiBatchEmbedding {
		var geminiResponse dto.GeminiBatchEmbeddingResponse
		err = common.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	} else {
		var geminiResponse dto.GeminiEmbeddingResponse
		err = common.Unmarshal(responseBody, &geminiResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return usage, nil
}

func GeminiTextGenerationStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)

	return geminiStreamHandler(c, info, resp, func(data string, geminiResponse *dto.GeminiChatResponse) bool {
		err := helper.StringData(c, data)
		if err != nil {
			logger.LogError(c, "failed to write stream data: "+err.Error())
			return false
		}
		info.SendResponseCount++
		return true
	})
}
