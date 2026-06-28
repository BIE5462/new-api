package service

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCleanGeneratedImageBase64(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte("image bytes"))

	assert.Equal(t, raw, CleanGeneratedImageBase64(" data:image/png;base64,"+raw+" \n"))
	assert.Equal(t, raw, CleanGeneratedImageBase64(raw[:4]+" \n\t"+raw[4:]))
	assert.Equal(t, raw, CleanGeneratedImageBase64(raw))
}

func TestEstimateBase64DecodedBytes(t *testing.T) {
	assert.Equal(t, int64(0), EstimateBase64DecodedBytes(""))
	assert.Equal(t, int64(1), EstimateBase64DecodedBytes(base64.StdEncoding.EncodeToString([]byte{1})))
	assert.Equal(t, int64(2), EstimateBase64DecodedBytes(base64.StdEncoding.EncodeToString([]byte{1, 2})))
	assert.Equal(t, int64(3), EstimateBase64DecodedBytes(base64.StdEncoding.EncodeToString([]byte{1, 2, 3})))
}

func TestBuildGeneratedImageObjectKey(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 30, 0, 0, time.UTC)

	key := BuildGeneratedImageObjectKey("gemini/generated", GeneratedImageUploadMeta{
		RequestID:      "req:with/slashes and spaces",
		CandidateIndex: 2,
		PartIndex:      3,
		MimeType:       "image/jpeg",
	}, now)

	assert.Equal(t, "gemini/generated/2026/06/28/req-with-slashes-and-spaces-2-3.jpg", key)
	assert.Equal(t, "gemini/generated/2026/06/28/request-0-0.bin", BuildGeneratedImageObjectKey("", GeneratedImageUploadMeta{}, now))
	assert.Equal(t, "prefix/2026/06/28/abc-0-1.webp", BuildGeneratedImageObjectKey("\\prefix\\", GeneratedImageUploadMeta{
		RequestID: "abc",
		PartIndex: 1,
		MimeType:  "image/webp",
	}, now))
}
