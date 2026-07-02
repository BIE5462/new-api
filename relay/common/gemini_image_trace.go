package common

import (
	"fmt"
	"strings"
	"time"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

const geminiImageTraceEnv = "GEMINI_IMAGE_TRACE_ENABLED"

var geminiImageTraceEnabled = rootcommon.GetEnvOrDefaultBool(geminiImageTraceEnv, false)

func GeminiImageTraceEnabled() bool {
	return geminiImageTraceEnabled
}

func IsGeminiImageTraceRequest(info *RelayInfo) bool {
	if !GeminiImageTraceEnabled() || info == nil {
		return false
	}
	if info.ChannelMeta == nil {
		return false
	}
	if info.ApiType != constant.APITypeGemini && info.ApiType != constant.APITypeVertexAi {
		return false
	}
	if info.RelayMode == relayconstant.RelayModeImagesGenerations ||
		info.RelayMode == relayconstant.RelayModeImagesEdits {
		return true
	}
	if info.RelayMode == relayconstant.RelayModeGemini &&
		!info.IsStream &&
		isGeminiImageModel(info.UpstreamModelName) &&
		(strings.Contains(info.RequestURLPath, ":generateContent") || strings.Contains(info.RequestURLPath, "/generateContent")) {
		return true
	}
	return false
}

func isGeminiImageModel(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(modelName, "image") ||
		strings.HasPrefix(modelName, "imagen") ||
		model_setting.IsGeminiModelSupportImagine(modelName)
}

func GeminiImageTrace(c *gin.Context, info *RelayInfo, event string, startedAt time.Time, args ...any) {
	if !IsGeminiImageTraceRequest(info) {
		return
	}
	var b strings.Builder
	b.WriteString("[gemini-image-trace]")
	b.WriteString(" event=")
	b.WriteString(event)
	if info != nil {
		if info.RequestId != "" {
			b.WriteString(" request_id=")
			b.WriteString(info.RequestId)
		}
		if info.RetryIndex > 0 {
			b.WriteString(fmt.Sprintf(" retry=%d", info.RetryIndex))
		}
		b.WriteString(fmt.Sprintf(" relay_mode=%d", info.RelayMode))
		if info.ApiType >= 0 {
			b.WriteString(fmt.Sprintf(" api_type=%d", info.ApiType))
		}
		if info.ChannelMeta != nil {
			b.WriteString(fmt.Sprintf(" channel_id=%d channel_type=%d", info.ChannelId, info.ChannelType))
			if info.ChannelIsMultiKey {
				b.WriteString(fmt.Sprintf(" multi_key_index=%d", info.ChannelMultiKeyIndex))
			}
		}
		if info.OriginModelName != "" {
			b.WriteString(" origin_model=")
			b.WriteString(info.OriginModelName)
		}
		if info.UpstreamModelName != "" {
			b.WriteString(" upstream_model=")
			b.WriteString(info.UpstreamModelName)
		}
		if info.RequestURLPath != "" {
			b.WriteString(" request_path=")
			b.WriteString(formatGeminiImageTraceValue(stripGeminiImageTraceQuery(info.RequestURLPath)))
		}
		if !info.StartTime.IsZero() {
			b.WriteString(fmt.Sprintf(" elapsed_total_ms=%d", time.Since(info.StartTime).Milliseconds()))
		}
	}
	if !startedAt.IsZero() {
		b.WriteString(fmt.Sprintf(" duration_ms=%d", time.Since(startedAt).Milliseconds()))
	}
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatGeminiImageTraceValue(args[i+1]))
	}
	logger.LogInfo(c, b.String())
}

func formatGeminiImageTraceValue(value any) string {
	switch v := value.(type) {
	case string:
		return sanitizeGeminiImageTraceString(v)
	case fmt.Stringer:
		return sanitizeGeminiImageTraceString(v.String())
	default:
		return sanitizeGeminiImageTraceString(fmt.Sprint(v))
	}
}

func sanitizeGeminiImageTraceString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " =") {
		return fmt.Sprintf("%q", value)
	}
	return value
}

func stripGeminiImageTraceQuery(value string) string {
	if idx := strings.Index(value, "?"); idx >= 0 {
		return value[:idx]
	}
	return value
}
