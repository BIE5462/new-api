package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

func KlingRequestConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		var originalReq map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &originalReq); err != nil {
			c.Next()
			return
		}

		originalPath := c.Request.URL.Path
		// Support both model_name and model fields
		model, _ := originalReq["model_name"].(string)
		if model == "" {
			model, _ = originalReq["model"].(string)
		}
		prompt, _ := originalReq["prompt"].(string)
		if isKlingOmniRequestModel(model) {
			logger.LogInfo(c, fmt.Sprintf(
				"Kling middleware original payload: path=%q model=%q keys=%q has_image=%t image_list={%s} metadata_image_list={%s}",
				originalPath,
				model,
				strings.Join(sortedKlingRequestKeys(originalReq), ","),
				nonEmptyKlingString(originalReq["image"]),
				klingRequestObjectListShape(originalReq["image_list"]),
				klingRequestObjectListShape(klingNestedMetadataValue(originalReq, "image_list")),
			))
		}

		unifiedReq := map[string]interface{}{
			"model":    model,
			"prompt":   prompt,
			"metadata": originalReq,
		}

		jsonData, err := json.Marshal(unifiedReq)
		if err != nil {
			c.Next()
			return
		}

		// Rewrite request body and path
		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		c.Request.URL.Path = "/v1/video/generations"
		switch {
		case strings.HasSuffix(originalPath, "/videos/omni-video"):
			c.Set("action", constant.TaskActionOmniVideo)
		case strings.HasSuffix(originalPath, "/videos/motion-control"):
			c.Set("action", constant.TaskActionMotionControl)
		case strings.HasSuffix(originalPath, "/videos/text2video"):
			c.Set("action", constant.TaskActionTextGenerate)
		case strings.HasSuffix(originalPath, "/videos/image2video"):
			c.Set("action", constant.TaskActionGenerate)
		default:
			if image, ok := originalReq["image"]; !ok || image == "" {
				c.Set("action", constant.TaskActionTextGenerate)
			}
		}

		// We have to reset the request body for the next handlers
		c.Set(common.KeyRequestBody, jsonData)
		c.Set(common.KeyBodyStorage, nil)
		c.Next()
	}
}

func isKlingOmniRequestModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "kling-v3-omni")
}

func nonEmptyKlingString(value any) bool {
	s, _ := value.(string)
	return strings.TrimSpace(s) != ""
}

func klingNestedMetadataValue(req map[string]any, key string) any {
	metadata, ok := req["metadata"].(map[string]any)
	if !ok {
		return nil
	}
	return metadata[key]
}

func klingRequestObjectListShape(value any) string {
	if value == nil {
		return "kind=<nil> count=0"
	}
	objects := normalizeKlingRequestObjectList(value)
	if len(objects) == 0 {
		return fmt.Sprintf("kind=%T count=0", value)
	}
	first := objects[0]
	firstType, _ := first["type"].(string)
	return fmt.Sprintf(
		"kind=%T count=%d first_keys=%q first_type=%q first_has_url=%t",
		value,
		len(objects),
		strings.Join(sortedKlingRequestKeys(first), ","),
		firstType,
		nonEmptyKlingString(first["image_url"]) || nonEmptyKlingString(first["url"]) || nonEmptyKlingString(first["image"]),
	)
}

func normalizeKlingRequestObjectList(value any) []map[string]any {
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
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil
		}
		return normalizeKlingRequestObjectList(decoded)
	case []map[string]any:
		return v
	case []any:
		list := make([]map[string]any, 0, len(v))
		for _, item := range v {
			object, ok := item.(map[string]any)
			if ok {
				list = append(list, object)
			}
		}
		return list
	case map[string]any:
		return []map[string]any{v}
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var list []map[string]any
		if err := json.Unmarshal(data, &list); err != nil {
			return nil
		}
		return list
	}
}

func sortedKlingRequestKeys(values map[string]any) []string {
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
