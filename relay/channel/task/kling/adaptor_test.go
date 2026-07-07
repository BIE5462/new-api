package kling

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

func withConfiguredKlingPrices(t *testing.T, pricesJSON string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("kling")
	original, err := config.ConfigToMap(cfg)
	if err != nil {
		t.Fatalf("snapshot kling config: %v", err)
	}
	if err := config.UpdateConfigFromMap(cfg, map[string]string{"prices": pricesJSON}); err != nil {
		t.Fatalf("update kling config: %v", err)
	}
	model_setting.RebuildKlingPriceIndex()
	t.Cleanup(func() {
		_ = config.UpdateConfigFromMap(cfg, map[string]string{"prices": original["prices"]})
		model_setting.RebuildKlingPriceIndex()
	})
}

func withQuotaPerUnit(t *testing.T, quotaPerUnit float64) {
	t.Helper()
	original := common.QuotaPerUnit
	common.QuotaPerUnit = quotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = original
	})
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	return c
}

func TestResolveTaskPriceUsesKlingCombinationAndDuration(t *testing.T) {
	withConfiguredKlingPrices(t, `[{"model":"kling-v1","mode":"pro","sound":"on","price_per_second":0.02}]`)
	withQuotaPerUnit(t, 1000)

	c := testGinContext()
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: "kling-v1",
		Metadata: map[string]interface{}{
			"mode":     " PRO ",
			"sound":    true,
			"duration": "10",
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "kling-v1",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, taskErr := (&TaskAdaptor{}).ResolveTaskPrice(c, info)
	if taskErr != nil {
		t.Fatalf("unexpected task error: %v", taskErr)
	}
	if !priceData.UsePrice {
		t.Fatal("expected kling price data to use fixed per-call price")
	}
	if priceData.Quota != 200 {
		t.Fatalf("expected quota 200, got %d", priceData.Quota)
	}
	if math.Abs(priceData.ModelPrice-0.2) > 1e-9 {
		t.Fatalf("expected total model price 0.2, got %v", priceData.ModelPrice)
	}
	if priceData.OtherRatios != nil {
		t.Fatalf("expected no OtherRatios for kling fixed price, got %#v", priceData.OtherRatios)
	}
	if got := priceData.BillingDetails["kling_mode"]; got != "pro" {
		t.Fatalf("expected kling_mode=pro, got %v", got)
	}
	if got := priceData.BillingDetails["kling_sound"]; got != "on" {
		t.Fatalf("expected kling_sound=on, got %v", got)
	}
	if got := priceData.BillingDetails["duration_seconds"]; got != float64(10) {
		t.Fatalf("expected duration_seconds=10, got %v", got)
	}
	gotPricePerSecond, ok := priceData.BillingDetails["price_per_second"].(float64)
	if !ok || math.Abs(gotPricePerSecond-0.02) > 1e-9 {
		t.Fatalf("expected price_per_second=0.02, got %v", priceData.BillingDetails["price_per_second"])
	}
}

func TestResolveTaskPriceReturnsModelPriceErrorWhenCombinationMissing(t *testing.T) {
	withConfiguredKlingPrices(t, `[]`)

	c := testGinContext()
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: "kling-v1",
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "kling-v1",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	_, taskErr := (&TaskAdaptor{}).ResolveTaskPrice(c, info)
	if taskErr == nil {
		t.Fatal("expected missing price error")
	}
	if taskErr.Code != "model_price_error" {
		t.Fatalf("expected model_price_error, got %s", taskErr.Code)
	}
	if taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", taskErr.StatusCode)
	}
}

func TestResolveKlingDurationSeconds(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		want    float64
		wantErr bool
	}{
		{
			name: "default",
			req:  relaycommon.TaskSubmitReq{},
			want: 5,
		},
		{
			name: "metadata number",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]interface{}{"duration": float64(6)},
			},
			want: 6,
		},
		{
			name: "metadata numeric string",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]interface{}{"duration": "7.5"},
			},
			want: 7.5,
		},
		{
			name: "top-level raw string",
			req: func() relaycommon.TaskSubmitReq {
				var req relaycommon.TaskSubmitReq
				if err := common.Unmarshal([]byte(`{"prompt":"x","model":"kling-v1","duration":"8"}`), &req); err != nil {
					t.Fatalf("unmarshal task req: %v", err)
				}
				return req
			}(),
			want: 8,
		},
		{
			name: "invalid string",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]interface{}{"duration": "abc"},
			},
			wantErr: true,
		},
		{
			name: "zero",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]interface{}{"duration": 0},
			},
			wantErr: true,
		},
		{
			name: "negative",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]interface{}{"duration": -1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKlingDurationSeconds(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestResolveKlingSoundPrefersSoundAndFallsBackToAudio(t *testing.T) {
	if got := resolveKlingSound(relaycommon.TaskSubmitReq{Sound: true, Audio: false}); got != "on" {
		t.Fatalf("expected sound to win over audio, got %q", got)
	}
	if got := resolveKlingSound(relaycommon.TaskSubmitReq{Audio: true}); got != "on" {
		t.Fatalf("expected audio fallback to normalize to on, got %q", got)
	}
	if got := resolveKlingSound(relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"keep_original_sound": "yes"}}); got != "on" {
		t.Fatalf("expected keep_original_sound=yes to normalize to on, got %q", got)
	}
	if got := resolveKlingSound(relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"keep_original_sound": "no"}}); got != "off" {
		t.Fatalf("expected keep_original_sound=no to normalize to off, got %q", got)
	}
	if got := resolveKlingSound(relaycommon.TaskSubmitReq{}); got != "off" {
		t.Fatalf("expected default sound off, got %q", got)
	}
}

func TestConvertToRequestPayloadUsesNormalizedDurationAndPassesSoundAudio(t *testing.T) {
	var req relaycommon.TaskSubmitReq
	if err := common.Unmarshal([]byte(`{"prompt":"x","model":"kling-v1","duration":"8.5","sound":true,"audio":"voice"}`), &req); err != nil {
		t.Fatalf("unmarshal task req: %v", err)
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v1",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.Duration != "8.5" {
		t.Fatalf("expected duration 8.5, got %q", payload.Duration)
	}
	if payload.Sound != true {
		t.Fatalf("expected sound passthrough true, got %v", payload.Sound)
	}
	if payload.Audio != "voice" {
		t.Fatalf("expected audio passthrough voice, got %v", payload.Audio)
	}
}

func TestKlingVideoPathSupportsOmniVideo(t *testing.T) {
	if got := klingVideoPath(constant.TaskActionOmniVideo); got != "/v1/videos/omni-video" {
		t.Fatalf("expected omni-video path, got %q", got)
	}
	if got := klingVideoPath(constant.TaskActionMotionControl); got != "/v1/videos/motion-control" {
		t.Fatalf("expected motion-control path, got %q", got)
	}
	if got := klingVideoPath(constant.TaskActionGenerate); got != "/v1/videos/image2video" {
		t.Fatalf("expected image2video path, got %q", got)
	}
	if got := klingVideoPath(constant.TaskActionTextGenerate); got != "/v1/videos/text2video" {
		t.Fatalf("expected text2video path, got %q", got)
	}
}

func TestConvertToRequestPayloadPassesOmniFieldsFromMetadata(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v3-omni",
		Metadata: map[string]interface{}{
			"image_list": []interface{}{
				map[string]interface{}{"image_url": "https://example.com/a.png", "type": "first_frame"},
			},
			"watermark_info": map[string]interface{}{"enabled": false},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v3-omni",
		ChannelBaseUrl:    "https://yunwu.ai",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.ImageList == nil {
		t.Fatal("expected image_list passthrough")
	}
	if payload.Image != "https://example.com/a.png" {
		t.Fatalf("expected first_frame to populate image compatibility field, got %q", payload.Image)
	}
	if payload.WatermarkInfo == nil {
		t.Fatal("expected watermark_info passthrough")
	}
}

func TestConvertToRequestPayloadPassesKlingDocumentFields(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v3-omni",
		Metadata: map[string]interface{}{
			"model":      "kling-v3-omni",
			"model_name": "kling-v3-omni",
			"image_list": []interface{}{
				map[string]interface{}{"image_url": "https://example.com/first.png", "type": "first_frame"},
			},
			"video_list": []interface{}{
				map[string]interface{}{"video_url": "https://example.com/ref.mp4", "refer_type": "feature", "keep_original_sound": "no"},
			},
			"element_list": []interface{}{
				map[string]interface{}{"element_id": "element_xxx"},
			},
			"multi_shot":   true,
			"shot_type":    "customize",
			"multi_prompt": []interface{}{map[string]interface{}{"index": float64(1), "prompt": "shot", "duration": "5"}},
			"aspect_ratio": "16:9",
			"watermark_info": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v3-omni",
		ApiKey:            "sk-test",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.ImageList == nil {
		t.Fatal("expected image_list passthrough")
	}
	if payload.Image != "https://example.com/first.png" {
		t.Fatalf("expected first_frame to populate image compatibility field, got %q", payload.Image)
	}
	if payload.VideoList == nil {
		t.Fatal("expected video_list passthrough")
	}
	if payload.ElementList == nil {
		t.Fatal("expected element_list passthrough")
	}
	if !payload.MultiShot {
		t.Fatal("expected multi_shot passthrough")
	}
	if payload.ShotType != "customize" {
		t.Fatalf("expected shot_type customize, got %q", payload.ShotType)
	}
	if payload.MultiPrompt == nil {
		t.Fatal("expected multi_prompt passthrough")
	}
	if payload.AspectRatio != "16:9" {
		t.Fatalf("expected aspect_ratio passthrough, got %q", payload.AspectRatio)
	}
}

func TestConvertToRequestPayloadPassesMotionControlFields(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v3",
		Metadata: map[string]interface{}{
			"model_name":            "kling-v3",
			"image_url":             "https://example.com/ref.png",
			"video_url":             "https://example.com/motion.mp4",
			"character_orientation": "image",
			"mode":                  "pro",
			"keep_original_sound":   "yes",
			"callback_url":          "https://example.com/callback",
			"external_task_id":      "biz-task-1",
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v3",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.ImageURL != "https://example.com/ref.png" {
		t.Fatalf("expected image_url passthrough, got %q", payload.ImageURL)
	}
	if payload.VideoURL != "https://example.com/motion.mp4" {
		t.Fatalf("expected video_url passthrough, got %q", payload.VideoURL)
	}
	if payload.CharacterOrientation != "image" {
		t.Fatalf("expected character_orientation image, got %q", payload.CharacterOrientation)
	}
	if payload.KeepOriginalSound != "yes" {
		t.Fatalf("expected keep_original_sound yes, got %q", payload.KeepOriginalSound)
	}
	if payload.CallbackUrl != "https://example.com/callback" {
		t.Fatalf("expected callback_url passthrough, got %q", payload.CallbackUrl)
	}
	if payload.ExternalTaskId != "biz-task-1" {
		t.Fatalf("expected external_task_id passthrough, got %q", payload.ExternalTaskId)
	}
}

func TestConvertToRequestPayloadUsesSingleOmniImageListItemAsCompatibilityImage(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v3-omni",
		Metadata: map[string]interface{}{
			"model_name": "kling-v3-omni",
			"image_list": []interface{}{
				map[string]interface{}{"image_url": "https://example.com/ref.png"},
			},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v3-omni",
		ApiKey:            "sk-test",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.Image != "https://example.com/ref.png" {
		t.Fatalf("expected single image_list item to populate image compatibility field, got %q", payload.Image)
	}
	if payload.AspectRatio != "" {
		t.Fatalf("expected no default aspect_ratio when compatibility image is present, got %q", payload.AspectRatio)
	}
}

func TestConvertToRequestPayloadParsesStringEncodedOmniImageList(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v3-omni",
		Metadata: map[string]interface{}{
			"model_name": "kling-v3-omni",
			"image_list": `[{"image_url":"https://example.com/ref.png","type":"first_frame"}]`,
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v3-omni",
		ChannelBaseUrl:    "https://yunwu.ai",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.Image != "https://example.com/ref.png" {
		t.Fatalf("expected string encoded image_list to populate image compatibility field, got %q", payload.Image)
	}
}

func TestKlingBuildRequestURLTrimsTrailingSlash(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://yunwu.ai/"}
	url, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{
		Action: constant.TaskActionOmniVideo,
		ApiKey: "sk-test",
	})
	if err != nil {
		t.Fatalf("build url: %v", err)
	}
	if url != "https://yunwu.ai/kling/v1/videos/omni-video" {
		t.Fatalf("unexpected url: %q", url)
	}
}

func TestConvertToRequestPayloadNormalizesKlingV26Model(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt: "x",
		Model:  "kling-v2.6",
		Metadata: map[string]interface{}{
			"model":      "kling-v2.6",
			"model_name": "kling-v2.6",
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req, &relaycommon.RelayInfo{
		UpstreamModelName: "kling-v2.6",
	})
	if err != nil {
		t.Fatalf("convert payload: %v", err)
	}
	if payload.Model != "kling-v2-6" || payload.ModelName != "kling-v2-6" {
		t.Fatalf("expected normalized kling-v2-6 model fields, got model=%q model_name=%q", payload.Model, payload.ModelName)
	}
}

func TestKlingActionResolvesFromModel(t *testing.T) {
	omniReq := relaycommon.TaskSubmitReq{Model: "kling-v3-omni"}
	if got := resolveKlingAction(constant.TaskActionGenerate, omniReq); got != constant.TaskActionOmniVideo {
		t.Fatalf("expected omni action, got %q", got)
	}
	imageReq := relaycommon.TaskSubmitReq{Model: "kling-v2.6"}
	if got := resolveKlingAction(constant.TaskActionOmniVideo, imageReq); got != constant.TaskActionGenerate {
		t.Fatalf("expected image2video action, got %q", got)
	}
	motionReq := relaycommon.TaskSubmitReq{Model: "kling-v3"}
	if got := resolveKlingAction(constant.TaskActionMotionControl, motionReq); got != constant.TaskActionMotionControl {
		t.Fatalf("expected motion-control action, got %q", got)
	}
}

func TestKlingPromptlessMultiShotValidation(t *testing.T) {
	body := `{
		"model": "kling-v3-omni",
		"metadata": {
			"model": "kling-v3-omni",
			"model_name": "kling-v3-omni",
			"duration": "10",
			"multi_shot": true,
			"shot_type": "customize",
			"multi_prompt": [
				{"index": 1, "prompt": "shot one", "duration": "5"},
				{"index": 2, "prompt": "shot two", "duration": "5"}
			]
		}
	}`
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/video/generations", io.NopCloser(strings.NewReader(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	if taskErr != nil {
		t.Fatalf("unexpected task error: %v", taskErr)
	}
	if info.Action != constant.TaskActionOmniVideo {
		t.Fatalf("expected omni action, got %q", info.Action)
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		t.Fatalf("get task request: %v", err)
	}
	if req.Prompt != "" {
		t.Fatalf("expected promptless multi-shot request to keep empty prompt, got %q", req.Prompt)
	}
}

func TestParseTaskResultSupportsDocumentStatusAndURLs(t *testing.T) {
	body := []byte(`{
		"id": "root_task",
		"status": "succeeded",
		"progress": 100,
		"task_result": {
			"videos": [
				{"video_url": "https://example.com/output.mp4"}
			]
		}
	}`)

	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult(body)
	if err != nil {
		t.Fatalf("parse task result: %v", err)
	}
	if taskInfo.Status != "SUCCESS" {
		t.Fatalf("expected success status, got %q", taskInfo.Status)
	}
	if taskInfo.TaskID != "root_task" {
		t.Fatalf("expected root task id, got %q", taskInfo.TaskID)
	}
	if taskInfo.Url != "https://example.com/output.mp4" {
		t.Fatalf("expected video url, got %q", taskInfo.Url)
	}
	if taskInfo.Progress != "100%" {
		t.Fatalf("expected progress 100%%, got %q", taskInfo.Progress)
	}
}

func TestDoResponseAcceptsRootID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"code":0,"id":"upstream_task"}`)),
	}

	taskID, _, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, &relaycommon.RelayInfo{
		PublicTaskID:    "task_public",
		OriginModelName: "kling-v3",
	})
	if taskErr != nil {
		t.Fatalf("unexpected task error: %v", taskErr)
	}
	if taskID != "upstream_task" {
		t.Fatalf("expected upstream id, got %q", taskID)
	}
}
