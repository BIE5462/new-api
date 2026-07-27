package model_setting

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const DefaultKlingDurationSeconds = 5

type KlingPriceItem struct {
	Model             string  `json:"model"`
	Mode              string  `json:"mode"`
	Sound             string  `json:"sound"`
	HasReferenceVideo *bool   `json:"has_reference_video,omitempty"`
	PricePerSecond    float64 `json:"price_per_second"`
}

type KlingSettings struct {
	Prices []KlingPriceItem `json:"prices"`
}

var klingSettings = KlingSettings{
	Prices: []KlingPriceItem{},
}

type klingPriceIndex struct {
	exact    map[string]KlingPriceItem
	wildcard map[string]KlingPriceItem
}

var currentKlingPriceIndex atomic.Pointer[klingPriceIndex]

func init() {
	config.GlobalConfig.Register("kling", &klingSettings)
	RebuildKlingPriceIndex()
}

func NormalizeKlingModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "kling-v2.6" {
		return "kling-v2-6"
	}
	return model
}

func NormalizeKlingMode(value any) string {
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	if mode == "" || mode == "<nil>" {
		return "std"
	}
	return mode
}

func NormalizeKlingSound(value any) string {
	switch v := value.(type) {
	case nil:
		return "off"
	case bool:
		if v {
			return "on"
		}
		return "off"
	case string:
		sound := strings.ToLower(strings.TrimSpace(v))
		if sound == "" {
			return "off"
		}
		return sound
	default:
		sound := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		if sound == "" || sound == "<nil>" {
			return "off"
		}
		return sound
	}
}

func klingPriceKey(model string, mode any, sound any) string {
	return NormalizeKlingModel(model) + "|" + NormalizeKlingMode(mode) + "|" + NormalizeKlingSound(sound)
}

func klingReferenceVideoPriceKey(model string, mode any, sound any, hasReferenceVideo bool) string {
	return fmt.Sprintf("%s|ref=%t", klingPriceKey(model, mode, sound), hasReferenceVideo)
}

// RebuildKlingPriceIndex rebuilds the exact-match lookup table from kling.prices.
// Duplicate keys are intentionally resolved by the last loaded item.
func RebuildKlingPriceIndex() {
	idx := &klingPriceIndex{
		exact:    make(map[string]KlingPriceItem, len(klingSettings.Prices)),
		wildcard: make(map[string]KlingPriceItem, len(klingSettings.Prices)),
	}

	for _, item := range klingSettings.Prices {
		model := NormalizeKlingModel(item.Model)
		mode := NormalizeKlingMode(item.Mode)
		sound := NormalizeKlingSound(item.Sound)
		if model == "" || mode == "" || sound == "" || item.PricePerSecond < 0 {
			continue
		}
		hasReferenceVideo := item.HasReferenceVideo
		if model != "kling-v3-omni" {
			hasReferenceVideo = nil
		}
		normalized := KlingPriceItem{
			Model:             model,
			Mode:              mode,
			Sound:             sound,
			HasReferenceVideo: hasReferenceVideo,
			PricePerSecond:    item.PricePerSecond,
		}
		if hasReferenceVideo != nil {
			idx.exact[klingReferenceVideoPriceKey(model, mode, sound, *hasReferenceVideo)] = normalized
			continue
		}
		idx.wildcard[klingPriceKey(model, mode, sound)] = normalized
	}

	currentKlingPriceIndex.Store(idx)
}

func GetKlingPrice(model string, mode any, sound any, hasReferenceVideo ...bool) (float64, bool) {
	idx := currentKlingPriceIndex.Load()
	if idx == nil {
		return 0, false
	}
	if len(hasReferenceVideo) > 0 {
		item, ok := idx.exact[klingReferenceVideoPriceKey(model, mode, sound, hasReferenceVideo[0])]
		if ok {
			return item.PricePerSecond, true
		}
	}
	item, ok := idx.wildcard[klingPriceKey(model, mode, sound)]
	if !ok {
		return 0, false
	}
	return item.PricePerSecond, true
}

func GetKlingPricesForModel(model string) []KlingPriceItem {
	model = NormalizeKlingModel(model)
	if model == "" {
		return nil
	}
	items := make([]KlingPriceItem, 0)
	for _, item := range klingSettings.Prices {
		itemModel := NormalizeKlingModel(item.Model)
		mode := NormalizeKlingMode(item.Mode)
		sound := NormalizeKlingSound(item.Sound)
		if itemModel != model || mode == "" || sound == "" || item.PricePerSecond < 0 {
			continue
		}
		hasReferenceVideo := item.HasReferenceVideo
		if itemModel != "kling-v3-omni" {
			hasReferenceVideo = nil
		}
		items = append(items, KlingPriceItem{
			Model:             itemModel,
			Mode:              mode,
			Sound:             sound,
			HasReferenceVideo: hasReferenceVideo,
			PricePerSecond:    item.PricePerSecond,
		})
	}
	return items
}
