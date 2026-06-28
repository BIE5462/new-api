package system_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	GeneratedImageStorageProviderAliyunOSS = "aliyun_oss"

	GeneratedImageStorageCredentialModeEnv        = "env"
	GeneratedImageStorageCredentialModeEcsRamRole = "ecs_ram_role"

	GeneratedImageStorageFailurePolicyFallbackInline = "fallback_inline"
	GeneratedImageStorageFailurePolicyFailRequest    = "fail_request"
)

type GeneratedImageStorageSettings struct {
	Enabled              bool   `json:"enabled"`
	Provider             string `json:"provider"`
	CredentialMode       string `json:"credential_mode"`
	EcsRamRoleName       string `json:"ecs_ram_role_name"`
	Bucket               string `json:"bucket"`
	Region               string `json:"region"`
	InternalEndpoint     string `json:"internal_endpoint"`
	ExternalEndpoint     string `json:"external_endpoint"`
	PublicBaseURL        string `json:"public_base_url"`
	PresignEnabled       bool   `json:"presign_enabled"`
	PresignTTLSeconds    int    `json:"presign_ttl_seconds"`
	ObjectPrefix         string `json:"object_prefix"`
	ThresholdMB          int    `json:"threshold_mb"`
	MaxImageMB           int    `json:"max_image_mb"`
	MaxTotalMB           int    `json:"max_total_mb"`
	MaxUploadConcurrency int    `json:"max_upload_concurrency"`
	UploadTimeoutSeconds int    `json:"upload_timeout_seconds"`
	FailurePolicy        string `json:"failure_policy"`
}

var generatedImageStorageSettings = GeneratedImageStorageSettings{
	Enabled:              false,
	Provider:             GeneratedImageStorageProviderAliyunOSS,
	CredentialMode:       GeneratedImageStorageCredentialModeEnv,
	EcsRamRoleName:       "",
	Bucket:               "",
	Region:               "",
	InternalEndpoint:     "",
	ExternalEndpoint:     "",
	PublicBaseURL:        "",
	PresignEnabled:       true,
	PresignTTLSeconds:    3600,
	ObjectPrefix:         "gemini/generated",
	ThresholdMB:          1,
	MaxImageMB:           64,
	MaxTotalMB:           128,
	MaxUploadConcurrency: 2,
	UploadTimeoutSeconds: 60,
	FailurePolicy:        GeneratedImageStorageFailurePolicyFallbackInline,
}

func init() {
	config.GlobalConfig.Register("generated_image_storage", &generatedImageStorageSettings)
}

func GetGeneratedImageStorageSettings() *GeneratedImageStorageSettings {
	return &generatedImageStorageSettings
}

func (s GeneratedImageStorageSettings) Normalized() GeneratedImageStorageSettings {
	if s.Provider == "" {
		s.Provider = GeneratedImageStorageProviderAliyunOSS
	}
	if s.CredentialMode == "" {
		s.CredentialMode = GeneratedImageStorageCredentialModeEnv
	}
	if s.PresignTTLSeconds <= 0 {
		s.PresignTTLSeconds = 3600
	}
	if s.PresignTTLSeconds > 604800 {
		s.PresignTTLSeconds = 604800
	}
	if s.ObjectPrefix == "" {
		s.ObjectPrefix = "gemini/generated"
	}
	if s.ThresholdMB <= 0 {
		s.ThresholdMB = 1
	}
	if s.MaxImageMB <= 0 {
		s.MaxImageMB = 64
	}
	if s.MaxTotalMB <= 0 {
		s.MaxTotalMB = 128
	}
	if s.MaxUploadConcurrency <= 0 {
		s.MaxUploadConcurrency = 1
	}
	if s.UploadTimeoutSeconds <= 0 {
		s.UploadTimeoutSeconds = 60
	}
	if s.FailurePolicy != GeneratedImageStorageFailurePolicyFallbackInline && s.FailurePolicy != GeneratedImageStorageFailurePolicyFailRequest {
		s.FailurePolicy = GeneratedImageStorageFailurePolicyFallbackInline
	}
	return s
}
