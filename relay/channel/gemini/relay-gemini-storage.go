package gemini

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

type geminiInlineImageUploadTask struct {
	part           *dto.GeminiPart
	candidateIndex int
	partIndex      int
	mimeType       string
	data           string
	estimatedBytes int64
	url            string
	key            string
	decodedBytes   int64
	err            error
}

var uploadGeneratedImage = service.UploadGeneratedImage

func offloadGeminiInlineImages(c *gin.Context, info *relaycommon.RelayInfo, response *dto.GeminiChatResponse) (bool, error) {
	cfg := system_setting.GetGeneratedImageStorageSettings().Normalized()
	if !cfg.Enabled || response == nil {
		return false, nil
	}

	startedAt := time.Now()
	thresholdBytes := generatedImageMegabytesToBytes(cfg.ThresholdMB)
	maxImageBytes := generatedImageMegabytesToBytes(cfg.MaxImageMB)
	maxTotalBytes := generatedImageMegabytesToBytes(cfg.MaxTotalMB)
	failRequest := cfg.FailurePolicy == system_setting.GeneratedImageStorageFailurePolicyFailRequest

	var tasks []*geminiInlineImageUploadTask
	var inlineImageCount int
	var skippedCount int
	var eligibleBytes int64

	for candidateIndex := range response.Candidates {
		for partIndex := range response.Candidates[candidateIndex].Content.Parts {
			part := &response.Candidates[candidateIndex].Content.Parts[partIndex]
			if part.InlineData == nil {
				continue
			}

			mimeType := strings.TrimSpace(part.InlineData.MimeType)
			if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
				continue
			}
			inlineImageCount++

			if strings.TrimSpace(part.InlineData.Data) == "" {
				skippedCount++
				continue
			}

			cleanData := service.CleanGeneratedImageBase64(part.InlineData.Data)
			estimatedBytes := service.EstimateBase64DecodedBytes(cleanData)
			if estimatedBytes <= thresholdBytes {
				skippedCount++
				continue
			}
			if estimatedBytes > maxImageBytes {
				err := fmt.Errorf("generated image exceeds max_image_mb: candidate=%d part=%d estimated_bytes=%d max_bytes=%d", candidateIndex, partIndex, estimatedBytes, maxImageBytes)
				if failRequest {
					return false, err
				}
				logger.LogWarn(c, "Gemini generated image storage skipped: "+err.Error())
				skippedCount++
				continue
			}
			if eligibleBytes+estimatedBytes > maxTotalBytes {
				err := fmt.Errorf("generated images exceed max_total_mb: candidate=%d part=%d estimated_total_bytes=%d max_bytes=%d", candidateIndex, partIndex, eligibleBytes+estimatedBytes, maxTotalBytes)
				if failRequest {
					return false, err
				}
				logger.LogWarn(c, "Gemini generated image storage skipped: "+err.Error())
				skippedCount++
				continue
			}

			eligibleBytes += estimatedBytes
			tasks = append(tasks, &geminiInlineImageUploadTask{
				part:           part,
				candidateIndex: candidateIndex,
				partIndex:      partIndex,
				mimeType:       mimeType,
				data:           cleanData,
				estimatedBytes: estimatedBytes,
			})
		}
	}

	if len(tasks) == 0 {
		logger.LogDebug(c, "Gemini generated image storage summary: inline_images=%d eligible=0 skipped=%d duration_ms=%d", inlineImageCount, skippedCount, time.Since(startedAt).Milliseconds())
		return false, nil
	}

	uploadCtx := context.Background()
	if c != nil && c.Request != nil {
		uploadCtx = c.Request.Context()
	}
	if failRequest {
		var cancel context.CancelFunc
		uploadCtx, cancel = context.WithCancel(uploadCtx)
		defer cancel()
		cancelUpload := cancel
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(task *geminiInlineImageUploadTask) {
				defer wg.Done()
				meta := service.GeneratedImageUploadMeta{
					RequestID:      generatedImageRequestID(c, info),
					CandidateIndex: task.candidateIndex,
					PartIndex:      task.partIndex,
					MimeType:       task.mimeType,
				}
				task.url, task.key, task.decodedBytes, task.err = uploadGeneratedImage(uploadCtx, meta, task.data)
				if task.err != nil {
					cancelUpload()
				}
			}(task)
		}
		wg.Wait()
	} else {
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(task *geminiInlineImageUploadTask) {
				defer wg.Done()
				meta := service.GeneratedImageUploadMeta{
					RequestID:      generatedImageRequestID(c, info),
					CandidateIndex: task.candidateIndex,
					PartIndex:      task.partIndex,
					MimeType:       task.mimeType,
				}
				task.url, task.key, task.decodedBytes, task.err = uploadGeneratedImage(uploadCtx, meta, task.data)
			}(task)
		}
		wg.Wait()
	}

	var firstErr error
	var uploadedCount int
	var failedCount int
	var uploadedBytes int64
	for _, task := range tasks {
		if task.err != nil {
			failedCount++
			if firstErr == nil {
				firstErr = task.err
			}
			continue
		}
		uploadedCount++
		uploadedBytes += task.decodedBytes
	}

	if failRequest && firstErr != nil {
		return false, fmt.Errorf("upload generated Gemini inline image failed: %w", firstErr)
	}

	changed := false
	for _, task := range tasks {
		if task.err != nil {
			logger.LogWarn(c, fmt.Sprintf(
				"Gemini generated image storage upload failed, fallback inline: candidate=%d part=%d estimated_bytes=%d err=%v",
				task.candidateIndex,
				task.partIndex,
				task.estimatedBytes,
				task.err,
			))
			continue
		}
		task.part.FileData = &dto.GeminiFileData{
			MimeType: task.mimeType,
			FileUri:  task.url,
		}
		task.part.InlineData = nil
		changed = true
		logger.LogDebug(c, "Gemini generated image stored: candidate=%d part=%d bytes=%d key=%s", task.candidateIndex, task.partIndex, task.decodedBytes, summarizeGeneratedImageObjectKey(task.key))
	}

	logger.LogDebug(
		c,
		"Gemini generated image storage summary: inline_images=%d eligible=%d uploaded=%d skipped=%d failed=%d bytes=%d duration_ms=%d",
		inlineImageCount,
		len(tasks),
		uploadedCount,
		skippedCount,
		failedCount,
		uploadedBytes,
		time.Since(startedAt).Milliseconds(),
	)

	return changed, nil
}

func generatedImageMegabytesToBytes(mb int) int64 {
	if mb <= 0 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

func generatedImageRequestID(c *gin.Context, info *relaycommon.RelayInfo) string {
	if info != nil && info.RequestId != "" {
		return info.RequestId
	}
	if c != nil {
		if requestID := strings.TrimSpace(c.GetString(common.RequestIdKey)); requestID != "" {
			return requestID
		}
	}
	return "request"
}

func geminiResponseDebugSummary(response *dto.GeminiChatResponse) (inlineImageCount int, inlineBase64Chars int, textPreview string) {
	if response == nil {
		return 0, 0, ""
	}
	var preview strings.Builder
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(part.InlineData.MimeType)), "image/") {
				inlineImageCount++
				inlineBase64Chars += len(part.InlineData.Data)
			}
			if part.Text == "" || preview.Len() >= 160 {
				continue
			}
			remaining := 160 - preview.Len()
			if len(part.Text) <= remaining {
				preview.WriteString(part.Text)
				continue
			}
			preview.WriteString(part.Text[:remaining])
		}
	}
	return inlineImageCount, inlineBase64Chars, preview.String()
}

func summarizeGeneratedImageObjectKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	parts := strings.Split(key, "/")
	last := parts[len(parts)-1]
	if len(parts) >= 4 {
		return strings.Join(parts[len(parts)-4:], "/")
	}
	if len(last) > 96 {
		return last[:96]
	}
	return key
}
