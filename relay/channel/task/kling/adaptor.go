package kling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

// ============================
// Request / Response structures
// ============================

type TrajectoryPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type DynamicMask struct {
	Mask         string            `json:"mask,omitempty"`
	Trajectories []TrajectoryPoint `json:"trajectories,omitempty"`
}

type CameraConfig struct {
	Horizontal float64 `json:"horizontal,omitempty"`
	Vertical   float64 `json:"vertical,omitempty"`
	Pan        float64 `json:"pan,omitempty"`
	Tilt       float64 `json:"tilt,omitempty"`
	Roll       float64 `json:"roll,omitempty"`
	Zoom       float64 `json:"zoom,omitempty"`
}

type CameraControl struct {
	Type   string        `json:"type,omitempty"`
	Config *CameraConfig `json:"config,omitempty"`
}

type requestPayload struct {
	Prompt               string         `json:"prompt,omitempty"`
	Image                string         `json:"image,omitempty"`
	ImageURL             string         `json:"image_url,omitempty"`
	ImageList            any            `json:"image_list,omitempty"`
	ImageTail            string         `json:"image_tail,omitempty"`
	VideoURL             string         `json:"video_url,omitempty"`
	VideoList            any            `json:"video_list,omitempty"`
	ElementList          any            `json:"element_list,omitempty"`
	NegativePrompt       string         `json:"negative_prompt,omitempty"`
	Mode                 string         `json:"mode,omitempty"`
	Duration             string         `json:"duration,omitempty"`
	Sound                any            `json:"sound,omitempty"`
	Audio                any            `json:"audio,omitempty"`
	AspectRatio          string         `json:"aspect_ratio,omitempty"`
	CharacterOrientation string         `json:"character_orientation,omitempty"`
	KeepOriginalSound    string         `json:"keep_original_sound,omitempty"`
	ModelName            string         `json:"model_name,omitempty"`
	Model                string         `json:"model,omitempty"` // Compatible with upstreams that only recognize "model"
	MultiShot            bool           `json:"multi_shot,omitempty"`
	ShotType             string         `json:"shot_type,omitempty"`
	MultiPrompt          any            `json:"multi_prompt,omitempty"`
	CfgScale             float64        `json:"cfg_scale,omitempty"`
	StaticMask           string         `json:"static_mask,omitempty"`
	DynamicMasks         []DynamicMask  `json:"dynamic_masks,omitempty"`
	CameraControl        *CameraControl `json:"camera_control,omitempty"`
	WatermarkInfo        any            `json:"watermark_info,omitempty"`
	CallbackUrl          string         `json:"callback_url,omitempty"`
	ExternalTaskId       string         `json:"external_task_id,omitempty"`
}

type responsePayload struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	TaskId      string `json:"task_id"`
	Id          string `json:"id"`
	RequestId   string `json:"request_id"`
	Status      string `json:"status"`
	Progress    any    `json:"progress"`
	VideoUrl    string `json:"video_url"`
	ResultUrl   string `json:"result_url"`
	Url         string `json:"url"`
	DownloadUrl string `json:"download_url"`
	State       string `json:"state"`
	TaskState   string `json:"task_state"`
	Reason      string `json:"reason"`
	Result      struct {
		VideoUrl string `json:"video_url"`
		Url      string `json:"url"`
	} `json:"result"`
	Content struct {
		VideoUrl string `json:"video_url"`
		Url      string `json:"url"`
	} `json:"content"`
	Metadata struct {
		Url      string `json:"url"`
		VideoUrl string `json:"video_url"`
	} `json:"metadata"`
	TaskResult struct {
		Videos []struct {
			Url      string `json:"url"`
			VideoUrl string `json:"video_url"`
		} `json:"videos"`
	} `json:"task_result"`
	Data struct {
		TaskId        string `json:"task_id"`
		Id            string `json:"id"`
		TaskStatus    string `json:"task_status"`
		Status        string `json:"status"`
		State         string `json:"state"`
		TaskState     string `json:"task_state"`
		TaskStatusMsg string `json:"task_status_msg"`
		StatusMsg     string `json:"status_msg"`
		ErrorMessage  string `json:"error_message"`
		Message       string `json:"message"`
		Msg           string `json:"msg"`
		Reason        string `json:"reason"`
		FailReason    string `json:"fail_reason"`
		FailureReason string `json:"failure_reason"`
		Detail        string `json:"detail"`
		Details       string `json:"details"`
		Progress      any    `json:"progress"`
		VideoUrl      string `json:"video_url"`
		ResultUrl     string `json:"result_url"`
		Url           string `json:"url"`
		DownloadUrl   string `json:"download_url"`
		Result        struct {
			VideoUrl string `json:"video_url"`
			Url      string `json:"url"`
		} `json:"result"`
		Content struct {
			VideoUrl string `json:"video_url"`
			Url      string `json:"url"`
		} `json:"content"`
		Metadata struct {
			Url      string `json:"url"`
			VideoUrl string `json:"video_url"`
		} `json:"metadata"`
		TaskInfo struct {
			ExternalTaskId string `json:"external_task_id"`
		} `json:"task_info"`
		WatermarkInfo struct {
			Enabled bool `json:"enabled"`
		} `json:"watermark_info"`
		TaskResult struct {
			Videos []struct {
				Id           string `json:"id"`
				Url          string `json:"url"`
				VideoUrl     string `json:"video_url"`
				WatermarkUrl string `json:"watermark_url"`
				Duration     string `json:"duration"`
			} `json:"videos"`
			Images []struct {
				Index        int    `json:"index"`
				Url          string `json:"url"`
				WatermarkUrl string `json:"watermark_url"`
			} `json:"images"`
		} `json:"task_result"`
		CreatedAt          int64  `json:"created_at"`
		UpdatedAt          int64  `json:"updated_at"`
		FinalUnitDeduction string `json:"final_unit_deduction"`
	} `json:"data"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey

	// apiKey format: "access_key|secret_key"
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Use the standard validation method for TaskSubmitReq
	action := c.GetString("action")
	if action == "" {
		action = constant.TaskActionGenerate
	}
	originalPromptlessMultiShotBody, restorePromptlessMultiShot := preparePromptlessKlingMultiShotValidation(c)
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, action); taskErr != nil {
		if restorePromptlessMultiShot {
			restoreKlingRequestBody(c, originalPromptlessMultiShotBody)
		}
		return taskErr
	}
	if req, err := relaycommon.GetTaskRequest(c); err == nil {
		if restorePromptlessMultiShot {
			req.Prompt = ""
			c.Set("task_request", req)
			restoreKlingRequestBody(c, originalPromptlessMultiShotBody)
		}
		info.Action = resolveKlingAction(action, req)
		c.Set("action", info.Action)
	}
	return nil
}

func (a *TaskAdaptor) ResolveTaskPrice(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, *dto.TaskError) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return types.PriceData{}, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	modelName := model_setting.NormalizeKlingModel(info.OriginModelName)
	if modelName == "" {
		modelName = resolveKlingModelFromRequest(req)
	}

	mode := resolveKlingMode(req)
	sound := resolveKlingSound(req)
	durationSeconds, err := resolveKlingDurationSeconds(req)
	if err != nil {
		return types.PriceData{}, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	pricePerSecond, ok := model_setting.GetKlingPrice(modelName, mode, sound)
	if !ok {
		return types.PriceData{}, service.TaskErrorWrapperLocal(
			fmt.Errorf("kling price not configured: model=%s, mode=%s, sound=%s", modelName, mode, sound),
			"model_price_error",
			http.StatusBadRequest,
		)
	}

	groupRatioInfo := helper.HandleGroupRatio(c, info)
	totalPrice := pricePerSecond * durationSeconds
	quota := int(totalPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
	freeModel := false
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		if groupRatioInfo.GroupRatio == 0 || totalPrice == 0 {
			quota = 0
			freeModel = true
		}
	}

	return types.PriceData{
		FreeModel:      freeModel,
		ModelPrice:     totalPrice,
		UsePrice:       true,
		Quota:          quota,
		GroupRatioInfo: groupRatioInfo,
		BillingDetails: map[string]interface{}{
			"price_per_second": pricePerSecond,
			"duration_seconds": durationSeconds,
			"kling_mode":       mode,
			"kling_sound":      sound,
		},
	}, nil
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	path := klingVideoPath(info.Action)
	baseURL := strings.TrimRight(a.baseURL, "/")

	if isNewAPIRelay(info.ApiKey) {
		return fmt.Sprintf("%s/kling%s", baseURL, path), nil
	}

	return fmt.Sprintf("%s%s", baseURL, path), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	token, err := a.createJWTToken()
	if err != nil {
		return fmt.Errorf("failed to create JWT token: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kling-sdk/1.0")
	return nil
}

// BuildRequestBody converts request into Kling specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)

	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	if isKlingOmniModel(body.ModelName, body.Model) {
		firstFrameImage := firstKlingImageURLByType(body.ImageList, "first_frame")
		singleImage := singleKlingImageURL(body.ImageList)
		logger.LogInfo(c, fmt.Sprintf(
			"Kling omni upstream payload: compat=%t has_image=%t has_first_frame=%t has_single_image=%t aspect_ratio=%q model=%q metadata_keys=%q metadata_image_list={%s} body_image_list={%s}",
			shouldApplyKlingOmniRelayCompatibility(info),
			body.Image != "",
			firstFrameImage != "",
			singleImage != "",
			body.AspectRatio,
			body.ModelName,
			strings.Join(sortedKlingMapKeys(req.Metadata), ","),
			klingObjectListShape(metadataValue(req.Metadata, "image_list")),
			klingObjectListShape(body.ImageList),
		))
	}
	if info.Action != constant.TaskActionOmniVideo && info.Action != constant.TaskActionMotionControl && body.Image == "" && body.ImageTail == "" {
		c.Set("action", constant.TaskActionTextGenerate)
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	if action := c.GetString("action"); action != "" {
		info.Action = action
	}
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	var kResp responsePayload
	err = common.Unmarshal(responseBody, &kResp)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if kResp.Code != 0 {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("%s", kResp.Message), "task_failed", http.StatusBadRequest)
		return
	}
	upstreamTaskID := firstNonEmptyString(kResp.TaskId, kResp.Id, kResp.Data.TaskId, kResp.Data.Id)
	if upstreamTaskID == "" {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("upstream response missing task_id"), "task_failed", http.StatusBadRequest)
		return
	}
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return upstreamTaskID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	action, ok := body["action"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid action")
	}
	path := klingVideoPath(action)
	baseUrl = strings.TrimRight(baseUrl, "/")
	url := fmt.Sprintf("%s%s/%s", baseUrl, path, taskID)
	if isNewAPIRelay(key) {
		url = fmt.Sprintf("%s/kling%s/%s", baseUrl, path, taskID)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	token, err := a.createJWTTokenWithKey(key)
	if err != nil {
		token = key
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kling-sdk/1.0")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{"kling-v1", "kling-v1-6", "kling-v2-master", "kling-v2-6", "kling-v3", "kling-v3-omni"}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "kling"
}

// ============================
// helpers
// ============================

func klingVideoPath(action string) string {
	switch action {
	case constant.TaskActionOmniVideo:
		return "/v1/videos/omni-video"
	case constant.TaskActionMotionControl:
		return "/v1/videos/motion-control"
	case constant.TaskActionGenerate:
		return "/v1/videos/image2video"
	default:
		return "/v1/videos/text2video"
	}
}

func resolveKlingAction(defaultAction string, req relaycommon.TaskSubmitReq) string {
	if defaultAction == constant.TaskActionMotionControl {
		return defaultAction
	}
	modelName := resolveKlingModelFromRequest(req)
	switch modelName {
	case "kling-v3-omni":
		return constant.TaskActionOmniVideo
	case "kling-v3", "kling-v2-6":
		return constant.TaskActionGenerate
	}
	return defaultAction
}

func preparePromptlessKlingMultiShotValidation(c *gin.Context) ([]byte, bool) {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return nil, false
	}
	if strings.TrimSpace(req.Prompt) != "" || !isKlingMultiShotRequest(req.Metadata) {
		return nil, false
	}
	originalData, err := common.Marshal(req)
	if err != nil {
		return nil, false
	}
	req.Prompt = firstKlingMultiPromptText(req.Metadata["multi_prompt"])
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, false
	}
	data, err := common.Marshal(req)
	if err != nil {
		return nil, false
	}
	c.Set(common.KeyRequestBody, data)
	c.Set(common.KeyBodyStorage, nil)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	return originalData, true
}

func restoreKlingRequestBody(c *gin.Context, data []byte) {
	if len(data) == 0 {
		return
	}
	c.Set(common.KeyRequestBody, data)
	c.Set(common.KeyBodyStorage, nil)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
}

func isKlingMultiShotRequest(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	multiShot, ok := metadata["multi_shot"].(bool)
	if !ok || !multiShot {
		return false
	}
	if prompt, ok := metadata["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		return true
	}
	return hasNonEmptyKlingMultiPrompt(metadata["multi_prompt"])
}

func firstKlingMultiPromptText(value any) string {
	prompts, ok := value.([]any)
	if !ok || len(prompts) == 0 {
		return ""
	}
	first, ok := prompts[0].(map[string]any)
	if !ok {
		return ""
	}
	prompt, _ := first["prompt"].(string)
	return strings.TrimSpace(prompt)
}

func hasNonEmptyKlingMultiPrompt(value any) bool {
	switch prompts := value.(type) {
	case []any:
		return len(prompts) > 0
	default:
		return false
	}
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	durationSeconds := float64(model_setting.DefaultKlingDurationSeconds)
	if seconds, err := resolveKlingDurationSeconds(*req); err == nil {
		durationSeconds = seconds
	}

	r := requestPayload{
		Prompt:         req.Prompt,
		Image:          req.Image,
		Mode:           taskcommon.DefaultString(req.Mode, "std"),
		Duration:       formatKlingDuration(durationSeconds),
		Sound:          req.Sound,
		Audio:          req.Audio,
		AspectRatio:    a.getAspectRatio(req.Size),
		ModelName:      info.UpstreamModelName,
		Model:          info.UpstreamModelName,
		CfgScale:       0.5,
		StaticMask:     "",
		DynamicMasks:   []DynamicMask{},
		CameraControl:  nil,
		CallbackUrl:    "",
		ExternalTaskId: "",
	}
	if r.ModelName == "" {
		r.ModelName = "kling-v1"
		r.Model = "kling-v1"
	}
	metadata := normalizeKlingRequestMetadata(req.Metadata)
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	r.ModelName = model_setting.NormalizeKlingModel(r.ModelName)
	r.Model = model_setting.NormalizeKlingModel(r.Model)
	if r.ModelName == "" {
		r.ModelName = r.Model
	}
	if r.Model == "" {
		r.Model = r.ModelName
	}
	applyKlingOmniCompatibility(&r, shouldApplyKlingOmniRelayCompatibility(info))
	return &r, nil
}

func resolveKlingModelFromRequest(req relaycommon.TaskSubmitReq) string {
	if modelName := model_setting.NormalizeKlingModel(req.Model); modelName != "" {
		return modelName
	}
	if req.Metadata != nil {
		if v, ok := req.Metadata["model_name"].(string); ok {
			if modelName := model_setting.NormalizeKlingModel(v); modelName != "" {
				return modelName
			}
		}
		if v, ok := req.Metadata["model"].(string); ok {
			return model_setting.NormalizeKlingModel(v)
		}
	}
	return ""
}

func resolveKlingMode(req relaycommon.TaskSubmitReq) string {
	if req.Metadata != nil {
		if v, ok := req.Metadata["mode"]; ok {
			return model_setting.NormalizeKlingMode(v)
		}
	}
	return model_setting.NormalizeKlingMode(req.Mode)
}

func resolveKlingSound(req relaycommon.TaskSubmitReq) string {
	if req.Metadata != nil {
		if v, ok := req.Metadata["sound"]; ok {
			return model_setting.NormalizeKlingSound(v)
		}
	}
	if req.Sound != nil {
		return model_setting.NormalizeKlingSound(req.Sound)
	}
	if req.Metadata != nil {
		if v, ok := req.Metadata["audio"]; ok {
			return model_setting.NormalizeKlingSound(v)
		}
	}
	if req.Metadata != nil {
		if v, ok := req.Metadata["keep_original_sound"]; ok {
			return normalizeKlingKeepOriginalSound(v)
		}
	}
	return model_setting.NormalizeKlingSound(req.Audio)
}

func normalizeKlingKeepOriginalSound(value any) string {
	sound := model_setting.NormalizeKlingSound(value)
	switch sound {
	case "yes":
		return "on"
	case "no":
		return "off"
	default:
		return sound
	}
}

func resolveKlingDurationSeconds(req relaycommon.TaskSubmitReq) (float64, error) {
	if req.Metadata != nil {
		if v, ok := req.Metadata["duration"]; ok {
			if v == nil {
				return model_setting.DefaultKlingDurationSeconds, nil
			}
			return parseKlingDurationValue(v)
		}
	}
	if len(req.DurationRaw) > 0 {
		return parseKlingDurationRaw(req.DurationRaw)
	}
	if req.Duration != 0 {
		return parseKlingDurationValue(req.Duration)
	}
	return model_setting.DefaultKlingDurationSeconds, nil
}

func parseKlingDurationRaw(raw []byte) (float64, error) {
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("duration is invalid")
	}
	if value == nil {
		return model_setting.DefaultKlingDurationSeconds, nil
	}
	return parseKlingDurationValue(value)
}

func parseKlingDurationValue(value any) (float64, error) {
	var seconds float64
	switch v := value.(type) {
	case int:
		seconds = float64(v)
	case int8:
		seconds = float64(v)
	case int16:
		seconds = float64(v)
	case int32:
		seconds = float64(v)
	case int64:
		seconds = float64(v)
	case uint:
		seconds = float64(v)
	case uint8:
		seconds = float64(v)
	case uint16:
		seconds = float64(v)
	case uint32:
		seconds = float64(v)
	case uint64:
		seconds = float64(v)
	case float32:
		seconds = float64(v)
	case float64:
		seconds = v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("duration is invalid")
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, fmt.Errorf("duration is invalid")
		}
		seconds = parsed
	default:
		return 0, fmt.Errorf("duration is invalid")
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0, fmt.Errorf("duration must be greater than 0")
	}
	return seconds, nil
}

func formatKlingDuration(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func normalizeKlingRequestMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	normalized := make(map[string]any, len(metadata))
	for key, value := range metadata {
		normalized[key] = value
	}
	if value, ok := normalized["duration"]; ok && value != nil {
		if seconds, err := parseKlingDurationValue(value); err == nil {
			normalized["duration"] = formatKlingDuration(seconds)
		}
	}
	return normalized
}

func shouldApplyKlingOmniRelayCompatibility(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return isNewAPIRelay(info.ApiKey) || strings.Contains(strings.ToLower(info.ChannelBaseUrl), "yunwu.ai")
}

func applyKlingOmniCompatibility(r *requestPayload, compatRelay bool) {
	if r == nil || !isKlingOmniModel(r.ModelName, r.Model) {
		return
	}
	if compatRelay && r.Image == "" {
		r.Image = firstKlingImageURLByType(r.ImageList, "first_frame")
		if r.Image == "" {
			r.Image = singleKlingImageURL(r.ImageList)
		}
	}
	if compatRelay && r.AspectRatio == "" && r.Image == "" && !hasKlingBaseVideo(r.VideoList) {
		r.AspectRatio = "1:1"
	}
}

func isKlingOmniModel(values ...string) bool {
	for _, value := range values {
		if model_setting.NormalizeKlingModel(value) == "kling-v3-omni" {
			return true
		}
	}
	return false
}

func firstKlingImageURLByType(imageList any, imageType string) string {
	images := normalizeKlingObjectList(imageList)
	for _, image := range images {
		currentType, _ := image["type"].(string)
		if !strings.EqualFold(strings.TrimSpace(currentType), imageType) {
			continue
		}
		if url := firstStringMapValue(image, "image_url", "url", "image"); url != "" {
			return url
		}
	}
	return ""
}

func singleKlingImageURL(imageList any) string {
	images := normalizeKlingObjectList(imageList)
	var onlyImage string
	for _, image := range images {
		url := firstStringMapValue(image, "image_url", "url", "image")
		if url == "" {
			continue
		}
		if onlyImage != "" {
			return ""
		}
		onlyImage = url
	}
	return onlyImage
}

func hasKlingBaseVideo(videoList any) bool {
	videos := normalizeKlingObjectList(videoList)
	for _, video := range videos {
		referType, _ := video["refer_type"].(string)
		switch strings.ToLower(strings.TrimSpace(referType)) {
		case "base", "video_editing", "video-editing":
			return true
		}
	}
	return false
}

func normalizeKlingObjectList(value any) []map[string]any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		var decoded any
		if err := common.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil
		}
		return normalizeKlingObjectList(decoded)
	case []map[string]any:
		return v
	case []any:
		list := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if object := normalizeKlingObject(item); object != nil {
				list = append(list, object)
			}
		}
		return list
	case map[string]any:
		return []map[string]any{v}
	}
	data, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	var list []map[string]any
	if err := common.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

func normalizeKlingObject(value any) map[string]any {
	if value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	data, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	var object map[string]any
	if err := common.Unmarshal(data, &object); err != nil {
		return nil
	}
	return object
}

func firstStringMapValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, _ := values[key].(string)
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func metadataValue(metadata map[string]any, key string) any {
	if metadata == nil {
		return nil
	}
	return metadata[key]
}

func klingObjectListShape(value any) string {
	if value == nil {
		return "kind=<nil> count=0"
	}
	objects := normalizeKlingObjectList(value)
	if len(objects) == 0 {
		return fmt.Sprintf("kind=%T count=0", value)
	}
	first := objects[0]
	firstType, _ := first["type"].(string)
	return fmt.Sprintf(
		"kind=%T count=%d first_keys=%q first_type=%q first_has_url=%t",
		value,
		len(objects),
		strings.Join(sortedKlingMapKeys(first), ","),
		firstType,
		firstStringMapValue(first, "image_url", "url", "image") != "",
	)
}

func sortedKlingMapKeys(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (a *TaskAdaptor) getAspectRatio(size string) string {
	switch size {
	case "1024x1024", "512x512":
		return "1:1"
	case "1280x720", "1920x1080":
		return "16:9"
	case "720x1280", "1080x1920":
		return "9:16"
	default:
		return ""
	}
}

// ============================
// JWT helpers
// ============================

func (a *TaskAdaptor) createJWTToken() (string, error) {
	return a.createJWTTokenWithKey(a.apiKey)
}

func (a *TaskAdaptor) createJWTTokenWithKey(apiKey string) (string, error) {
	if isNewAPIRelay(apiKey) {
		return apiKey, nil // new api relay
	}
	keyParts := strings.Split(apiKey, "|")
	if len(keyParts) != 2 {
		return "", errors.New("invalid api_key, required format is accessKey|secretKey")
	}
	accessKey := strings.TrimSpace(keyParts[0])
	if len(keyParts) == 1 {
		return accessKey, nil
	}
	secretKey := strings.TrimSpace(keyParts[1])
	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"iss": accessKey,
		"exp": now + 1800, // 30 minutes
		"nbf": now - 5,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = "JWT"
	return token.SignedString([]byte(secretKey))
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	taskInfo := &relaycommon.TaskInfo{}
	resPayload := responsePayload{}
	err := common.Unmarshal(respBody, &resPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}
	taskInfo.Code = resPayload.Code
	taskInfo.TaskID = firstNonEmptyString(resPayload.Data.TaskId, resPayload.Data.Id, resPayload.TaskId, resPayload.Id)
	taskInfo.Reason = extractKlingFailureReason(resPayload)
	taskInfo.Progress = formatKlingProgress(resPayload.Data.Progress)
	if taskInfo.Progress == "" {
		taskInfo.Progress = formatKlingProgress(resPayload.Progress)
	}
	status := normalizeKlingTaskStatus(firstNonEmptyString(
		resPayload.Data.TaskStatus,
		resPayload.Data.Status,
		resPayload.Data.State,
		resPayload.Data.TaskState,
		resPayload.Status,
		resPayload.State,
		resPayload.TaskState,
	))
	switch status {
	case model.TaskStatusSubmitted:
		taskInfo.Status = model.TaskStatusSubmitted
	case model.TaskStatusQueued:
		taskInfo.Status = model.TaskStatusQueued
	case model.TaskStatusInProgress:
		taskInfo.Status = model.TaskStatusInProgress
	case model.TaskStatusSuccess:
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Url = extractKlingVideoURL(resPayload)
		if tokens, err := strconv.ParseFloat(resPayload.Data.FinalUnitDeduction, 64); err == nil {
			rounded := int(math.Ceil(tokens))
			if rounded > 0 {
				taskInfo.CompletionTokens = rounded
				taskInfo.TotalTokens = rounded
			}
		}
	case model.TaskStatusFailure:
		taskInfo.Status = model.TaskStatusFailure
	default:
		return nil, fmt.Errorf("unknown task status: %s", firstNonEmptyString(resPayload.Data.TaskStatus, resPayload.Data.Status, resPayload.Data.State, resPayload.Data.TaskState, resPayload.Status, resPayload.State, resPayload.TaskState))
	}
	return taskInfo, nil
}

func isNewAPIRelay(apiKey string) bool {
	return strings.HasPrefix(apiKey, "sk-")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeKlingTaskStatus(status string) model.TaskStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "submitted", "submit", "created":
		return model.TaskStatusSubmitted
	case "queued", "queue", "pending", "waiting":
		return model.TaskStatusQueued
	case "processing", "running", "in_progress", "in-progress", "generating":
		return model.TaskStatusInProgress
	case "succeed", "succeeded", "success", "completed", "done", "finished":
		return model.TaskStatusSuccess
	case "fail", "failed", "failure", "error", "expired", "timeout", "timed_out", "cancel", "canceled", "cancelled", "rejected":
		return model.TaskStatusFailure
	default:
		return model.TaskStatusUnknown
	}
}

func extractKlingFailureReason(res responsePayload) string {
	return firstNonEmptyString(
		res.Data.FailReason,
		res.Data.FailureReason,
		res.Data.TaskStatusMsg,
		res.Data.StatusMsg,
		res.Data.ErrorMessage,
		res.Data.Message,
		res.Data.Msg,
		res.Data.Reason,
		res.Data.Detail,
		res.Data.Details,
		res.Reason,
		res.Message,
	)
}

func extractKlingVideoURL(res responsePayload) string {
	var videoURL string
	if videos := res.Data.TaskResult.Videos; len(videos) > 0 {
		videoURL = firstNonEmptyString(videos[0].Url, videos[0].VideoUrl)
	}
	var rootVideoURL string
	if videos := res.TaskResult.Videos; len(videos) > 0 {
		rootVideoURL = firstNonEmptyString(videos[0].Url, videos[0].VideoUrl)
	}
	return firstNonEmptyString(
		res.VideoUrl,
		res.ResultUrl,
		res.Url,
		res.DownloadUrl,
		res.Result.VideoUrl,
		res.Result.Url,
		res.Content.VideoUrl,
		res.Content.Url,
		rootVideoURL,
		res.Data.VideoUrl,
		res.Data.ResultUrl,
		res.Data.Url,
		res.Data.DownloadUrl,
		res.Data.Result.VideoUrl,
		res.Data.Result.Url,
		res.Data.Content.VideoUrl,
		res.Data.Content.Url,
		videoURL,
		res.Metadata.Url,
		res.Metadata.VideoUrl,
		res.Data.Metadata.Url,
		res.Data.Metadata.VideoUrl,
	)
}

func formatKlingProgress(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		progress := strings.TrimSpace(v)
		if progress == "" {
			return ""
		}
		if strings.HasSuffix(progress, "%") {
			return progress
		}
		if _, err := strconv.ParseFloat(progress, 64); err == nil {
			return progress + "%"
		}
		return progress
	case float64:
		return formatKlingProgressNumber(v)
	case float32:
		return formatKlingProgressNumber(float64(v))
	case int:
		return fmt.Sprintf("%d%%", v)
	case int64:
		return fmt.Sprintf("%d%%", v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return formatKlingProgressNumber(f)
		}
	}
	return ""
}

func formatKlingProgressNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64) + "%"
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var klingResp responsePayload
	if err := common.Unmarshal(originTask.Data, &klingResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal kling task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = klingResp.Data.CreatedAt
	openAIVideo.CompletedAt = klingResp.Data.UpdatedAt

	if videoURL := extractKlingVideoURL(klingResp); videoURL != "" {
		openAIVideo.SetMetadata("url", videoURL)
	}
	if len(klingResp.Data.TaskResult.Videos) > 0 {
		if duration := klingResp.Data.TaskResult.Videos[0].Duration; duration != "" {
			openAIVideo.Seconds = duration
		}
	}

	if klingResp.Code != 0 && klingResp.Message != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: klingResp.Message,
			Code:    fmt.Sprintf("%d", klingResp.Code),
		}
	}

	// https://app.klingai.com/cn/dev/document-api/apiReference/model/textToVideo
	if data := klingResp.Data; normalizeKlingTaskStatus(firstNonEmptyString(data.TaskStatus, data.Status, data.State, data.TaskState)) == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: extractKlingFailureReason(klingResp),
		}
	}
	return common.Marshal(openAIVideo)
}
