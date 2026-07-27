package model_setting

import "testing"

func withKlingPrices(t *testing.T, prices []KlingPriceItem) {
	t.Helper()
	original := klingSettings.Prices
	klingSettings.Prices = prices
	RebuildKlingPriceIndex()
	t.Cleanup(func() {
		klingSettings.Prices = original
		RebuildKlingPriceIndex()
	})
}

func TestKlingPriceIndexExactMatchAndReload(t *testing.T) {
	withKlingPrices(t, []KlingPriceItem{
		{Model: "kling-v1", Mode: "std", Sound: "off", PricePerSecond: 0.02},
		{Model: "kling-v1", Mode: "STD", Sound: "OFF", PricePerSecond: 0.03},
		{Model: "kling-v1", Mode: "std", Sound: "on", PricePerSecond: 0.04},
	})

	price, ok := GetKlingPrice("kling-v1", "std", "off")
	if !ok {
		t.Fatal("expected exact kling price")
	}
	if price != 0.03 {
		t.Fatalf("expected duplicate key to use last loaded price 0.03, got %v", price)
	}

	price, ok = GetKlingPrice("kling-v1", "std", "off", false)
	if !ok {
		t.Fatal("expected wildcard kling price for reference-video false")
	}
	if price != 0.03 {
		t.Fatalf("expected wildcard price 0.03, got %v", price)
	}

	price, ok = GetKlingPrice("kling-v1", "std", true)
	if !ok {
		t.Fatal("expected boolean sound=true to match sound=on")
	}
	if price != 0.04 {
		t.Fatalf("expected sound=on price 0.04, got %v", price)
	}

	klingSettings.Prices = []KlingPriceItem{}
	RebuildKlingPriceIndex()
	if _, ok := GetKlingPrice("kling-v1", "std", "off"); ok {
		t.Fatal("expected removed price to disappear after index rebuild")
	}
}

func TestKlingPriceIndexReferenceVideoExactOverridesWildcard(t *testing.T) {
	hasReferenceVideo := true
	noReferenceVideo := false
	withKlingPrices(t, []KlingPriceItem{
		{Model: "kling-v3-omni", Mode: "std", Sound: "off", PricePerSecond: 0.01},
		{Model: "kling-v3-omni", Mode: "std", Sound: "off", HasReferenceVideo: &hasReferenceVideo, PricePerSecond: 0.02},
		{Model: "kling-v3-omni", Mode: "std", Sound: "off", HasReferenceVideo: &noReferenceVideo, PricePerSecond: 0.03},
	})

	price, ok := GetKlingPrice("kling-v3-omni", "std", "off", true)
	if !ok || price != 0.02 {
		t.Fatalf("expected has reference video price 0.02, got price=%v ok=%t", price, ok)
	}

	price, ok = GetKlingPrice("kling-v3-omni", "std", "off", false)
	if !ok || price != 0.03 {
		t.Fatalf("expected no reference video price 0.03, got price=%v ok=%t", price, ok)
	}

	price, ok = GetKlingPrice("kling-v3-omni", "std", "off")
	if !ok || price != 0.01 {
		t.Fatalf("expected wildcard price 0.01, got price=%v ok=%t", price, ok)
	}
}

func TestKlingPriceIndexReferenceVideoFallbackToWildcard(t *testing.T) {
	withKlingPrices(t, []KlingPriceItem{
		{Model: "kling-v3-omni", Mode: "std", Sound: "off", PricePerSecond: 0.01},
	})

	price, ok := GetKlingPrice("kling-v3-omni", "std", "off", true)
	if !ok || price != 0.01 {
		t.Fatalf("expected has reference video to fall back to wildcard price 0.01, got price=%v ok=%t", price, ok)
	}

	price, ok = GetKlingPrice("kling-v3-omni", "std", "off", false)
	if !ok || price != 0.01 {
		t.Fatalf("expected no reference video to fall back to wildcard price 0.01, got price=%v ok=%t", price, ok)
	}
}

func TestKlingPriceIndexIgnoresReferenceVideoForNonOmniModels(t *testing.T) {
	hasReferenceVideo := true
	withKlingPrices(t, []KlingPriceItem{
		{Model: "kling-v1", Mode: "std", Sound: "off", HasReferenceVideo: &hasReferenceVideo, PricePerSecond: 0.02},
	})

	price, ok := GetKlingPrice("kling-v1", "std", "off")
	if !ok || price != 0.02 {
		t.Fatalf("expected non-omni reference-video row to behave as wildcard price 0.02, got price=%v ok=%t", price, ok)
	}

	price, ok = GetKlingPrice("kling-v1", "std", "off", true)
	if !ok || price != 0.02 {
		t.Fatalf(
			"expected non-omni reference-video lookup to fall back to wildcard price 0.02, got price=%v ok=%t",
			price,
			ok,
		)
	}
}

func TestKlingPriceIndexSkipsInvalidReferenceVideoRows(t *testing.T) {
	hasReferenceVideo := true
	withKlingPrices(t, []KlingPriceItem{
		{Model: "", Mode: "std", Sound: "off", HasReferenceVideo: &hasReferenceVideo, PricePerSecond: 0.01},
		{Model: "kling-v3-omni", Mode: "std", Sound: "off", HasReferenceVideo: &hasReferenceVideo, PricePerSecond: -0.01},
	})

	if _, ok := GetKlingPrice("kling-v3-omni", "std", "off", true); ok {
		t.Fatal("expected invalid reference-video rows to be skipped")
	}
}

func TestKlingRequestDimensionNormalization(t *testing.T) {
	if got := NormalizeKlingMode(""); got != "std" {
		t.Fatalf("expected empty mode to default to std, got %q", got)
	}
	if got := NormalizeKlingMode(" PRO "); got != "pro" {
		t.Fatalf("expected lowercase trimmed mode, got %q", got)
	}
	if got := NormalizeKlingSound(nil); got != "off" {
		t.Fatalf("expected nil sound to default to off, got %q", got)
	}
	if got := NormalizeKlingSound(false); got != "off" {
		t.Fatalf("expected false sound to normalize to off, got %q", got)
	}
	if got := NormalizeKlingSound(true); got != "on" {
		t.Fatalf("expected true sound to normalize to on, got %q", got)
	}
	if got := NormalizeKlingSound(" Voice "); got != "voice" {
		t.Fatalf("expected string sound to be lowercase trimmed, got %q", got)
	}
}
