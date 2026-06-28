package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type GeneratedImageUploadMeta struct {
	RequestID      string
	CandidateIndex int
	PartIndex      int
	MimeType       string
}

type generatedImageStorageClients struct {
	fingerprint    string
	internalClient *oss.Client
	externalClient *oss.Client
}

type countingReader struct {
	reader io.Reader
	n      int64
}

var (
	generatedImageStorageClientsMu sync.Mutex
	generatedImageStorageClientsV  *generatedImageStorageClients

	generatedImageUploadSemMu   sync.Mutex
	generatedImageUploadSem     chan struct{}
	generatedImageUploadSemSize int
)

func UploadGeneratedImage(ctx context.Context, meta GeneratedImageUploadMeta, base64Data string) (url string, key string, decodedBytes int64, err error) {
	cfg := system_setting.GetGeneratedImageStorageSettings().Normalized()
	if !cfg.Enabled {
		return "", "", 0, errors.New("generated image storage is disabled")
	}
	if cfg.Provider != system_setting.GeneratedImageStorageProviderAliyunOSS {
		return "", "", 0, fmt.Errorf("unsupported generated image storage provider: %s", cfg.Provider)
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return "", "", 0, errors.New("generated image storage bucket is empty")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return "", "", 0, errors.New("generated image storage region is empty")
	}

	cleanData := CleanGeneratedImageBase64(base64Data)
	decodedBytes = EstimateBase64DecodedBytes(cleanData)
	if decodedBytes <= 0 {
		return "", "", 0, errors.New("generated image base64 data is empty")
	}

	release, err := acquireGeneratedImageUploadSlot(ctx, cfg.MaxUploadConcurrency)
	if err != nil {
		return "", "", 0, err
	}
	defer release()

	uploadCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.UploadTimeoutSeconds)*time.Second)
	defer cancel()

	clients, err := getGeneratedImageStorageClients(cfg)
	if err != nil {
		return "", "", 0, err
	}

	key = BuildGeneratedImageObjectKey(cfg.ObjectPrefix, meta, time.Now().UTC())
	reader := &countingReader{reader: base64.NewDecoder(base64.StdEncoding, strings.NewReader(cleanData))}
	_, err = clients.internalClient.PutObject(uploadCtx, &oss.PutObjectRequest{
		Bucket:        oss.Ptr(cfg.Bucket),
		Key:           oss.Ptr(key),
		ContentType:   oss.Ptr(meta.MimeType),
		ContentLength: oss.Ptr(decodedBytes),
		Body:          reader,
	})
	if err != nil {
		return "", key, reader.n, err
	}

	url, err = buildGeneratedImageDownloadURL(uploadCtx, cfg, clients.externalClient, key)
	if err != nil {
		return "", key, reader.n, err
	}
	return url, key, reader.n, nil
}

func CleanGeneratedImageBase64(data string) string {
	data = strings.TrimSpace(data)
	if idx := strings.Index(data, ","); idx >= 0 {
		prefix := strings.ToLower(data[:idx])
		if strings.Contains(prefix, "base64") {
			data = data[idx+1:]
		}
	}
	if strings.IndexFunc(data, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\r' || r == '\t'
	}) < 0 {
		return data
	}
	var b strings.Builder
	b.Grow(len(data))
	for _, r := range data {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func EstimateBase64DecodedBytes(data string) int64 {
	if data == "" {
		return 0
	}
	n := base64.StdEncoding.DecodedLen(len(data))
	if strings.HasSuffix(data, "==") {
		n -= 2
	} else if strings.HasSuffix(data, "=") {
		n--
	}
	if n < 0 {
		return 0
	}
	return int64(n)
}

func BuildGeneratedImageObjectKey(prefix string, meta GeneratedImageUploadMeta, now time.Time) string {
	prefix = strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
	if prefix == "" {
		prefix = "gemini/generated"
	}
	requestID := sanitizeObjectKeyPart(meta.RequestID)
	if requestID == "" {
		requestID = "request"
	}
	ext := extensionFromMimeType(meta.MimeType)
	filename := fmt.Sprintf("%s-%d-%d.%s", requestID, meta.CandidateIndex, meta.PartIndex, ext)
	return path.Join(prefix, now.Format("2006/01/02"), filename)
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func getGeneratedImageStorageClients(cfg system_setting.GeneratedImageStorageSettings) (*generatedImageStorageClients, error) {
	fingerprint := generatedImageStorageFingerprint(cfg)
	generatedImageStorageClientsMu.Lock()
	defer generatedImageStorageClientsMu.Unlock()

	if generatedImageStorageClientsV != nil && generatedImageStorageClientsV.fingerprint == fingerprint {
		return generatedImageStorageClientsV, nil
	}

	provider, err := generatedImageStorageCredentialsProvider(cfg)
	if err != nil {
		return nil, err
	}
	internalEndpoint := cfg.InternalEndpoint
	if strings.TrimSpace(internalEndpoint) == "" {
		internalEndpoint = fmt.Sprintf("oss-%s-internal.aliyuncs.com", cfg.Region)
	}
	externalEndpoint := cfg.ExternalEndpoint
	if strings.TrimSpace(externalEndpoint) == "" {
		externalEndpoint = fmt.Sprintf("oss-%s.aliyuncs.com", cfg.Region)
	}

	internalCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(cfg.Region).
		WithEndpoint(internalEndpoint)
	externalCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(cfg.Region).
		WithEndpoint(externalEndpoint)

	clients := &generatedImageStorageClients{
		fingerprint:    fingerprint,
		internalClient: oss.NewClient(internalCfg),
		externalClient: oss.NewClient(externalCfg),
	}
	generatedImageStorageClientsV = clients
	return clients, nil
}

func generatedImageStorageCredentialsProvider(cfg system_setting.GeneratedImageStorageSettings) (credentials.CredentialsProvider, error) {
	switch cfg.CredentialMode {
	case system_setting.GeneratedImageStorageCredentialModeEnv:
		return credentials.NewEnvironmentVariableCredentialsProvider(), nil
	case system_setting.GeneratedImageStorageCredentialModeEcsRamRole:
		if strings.TrimSpace(cfg.EcsRamRoleName) == "" {
			return credentials.NewEcsRoleCredentialsProvider(), nil
		}
		return credentials.NewEcsRoleCredentialsProvider(credentials.EcsRamRole(cfg.EcsRamRoleName)), nil
	default:
		return nil, fmt.Errorf("unsupported generated image storage credential mode: %s", cfg.CredentialMode)
	}
}

func generatedImageStorageFingerprint(cfg system_setting.GeneratedImageStorageSettings) string {
	return strings.Join([]string{
		cfg.Provider,
		cfg.CredentialMode,
		cfg.EcsRamRoleName,
		cfg.Bucket,
		cfg.Region,
		cfg.InternalEndpoint,
		cfg.ExternalEndpoint,
		cfg.PublicBaseURL,
		fmt.Sprintf("%t", cfg.PresignEnabled),
		fmt.Sprintf("%d", cfg.PresignTTLSeconds),
	}, "\x1f")
}

func acquireGeneratedImageUploadSlot(ctx context.Context, concurrency int) (func(), error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	generatedImageUploadSemMu.Lock()
	if generatedImageUploadSem == nil || generatedImageUploadSemSize != concurrency {
		generatedImageUploadSem = make(chan struct{}, concurrency)
		generatedImageUploadSemSize = concurrency
	}
	sem := generatedImageUploadSem
	generatedImageUploadSemMu.Unlock()

	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func buildGeneratedImageDownloadURL(ctx context.Context, cfg system_setting.GeneratedImageStorageSettings, externalClient *oss.Client, key string) (string, error) {
	if strings.TrimSpace(cfg.PublicBaseURL) != "" {
		return joinPublicBaseURL(cfg.PublicBaseURL, key), nil
	}
	if !cfg.PresignEnabled {
		return joinOSSExternalURL(cfg.Bucket, cfg.ExternalEndpoint, cfg.Region, key), nil
	}
	result, err := externalClient.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(cfg.Bucket),
		Key:    oss.Ptr(key),
	}, func(o *oss.PresignOptions) {
		o.Expires = time.Duration(cfg.PresignTTLSeconds) * time.Second
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func joinPublicBaseURL(baseURL string, key string) string {
	return strings.TrimRight(baseURL, "/") + "/" + escapeObjectKeyPath(key)
}

func joinOSSExternalURL(bucket string, endpoint string, region string, key string) string {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = fmt.Sprintf("oss-%s.aliyuncs.com", region)
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return strings.TrimRight(endpoint, "/") + "/" + escapeObjectKeyPath(key)
	}
	parsed.Host = bucket + "." + parsed.Host
	parsed.Path = "/" + escapeObjectKeyPath(key)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func escapeObjectKeyPath(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func sanitizeObjectKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 96 {
		result = result[:96]
	}
	return result
}

func extensionFromMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch mimeType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/heic":
		return "heic"
	case "image/heif":
		return "heif"
	default:
		return "bin"
	}
}
