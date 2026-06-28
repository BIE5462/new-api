以下是基于 **OSS Go SDK V2** 的完整调用示例，涵盖 ECS 内网上传与公网预签名 URL 生成。

### 📦 完整代码示例

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func main() {
	// 基础配置
	bucketName := "bie2"
	region := "cn-guangzhou"
	objectKey := "images/user_001/photo.jpg"
	localFilePath := "./test_image.jpg" // 替换为实际图片路径

	// ==========================================
	// 1. 创建内网 Client（用于 ECS 实际传输数据）
	// ==========================================
	internalCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region).
		WithEndpoint("oss-cn-guangzhou-internal.aliyuncs.com") // 内网 Endpoint
	internalClient := oss.NewClient(internalCfg)

	// 2. 上传文件到 OSS
	file, err := os.Open(localFilePath)
	if err != nil {
		log.Fatalf("打开本地文件失败: %v", err)
	}
	defer file.Close()

	_, err = internalClient.PutObject(context.TODO(), &oss.PutObjectRequest{
		Bucket: oss.Ptr(bucketName),
		Key:    oss.Ptr(objectKey),
		Body:   file,
	})
	if err != nil {
		log.Fatalf("上传文件失败: %v", err)
	}
	fmt.Println("✅ 文件已通过内网成功上传至 OSS")

	// ==========================================
	// 3. 创建外网 Client（仅用于生成预签名 URL）
	// ==========================================
	externalCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region).
		WithEndpoint("oss-cn-guangzhou.aliyuncs.com") // 外网 Endpoint
	externalClient := oss.NewClient(externalCfg)

	// 4. 生成预签名下载 URL（有效期 1 小时）
	presignResult, err := externalClient.Presign(context.TODO(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucketName),
		Key:    oss.Ptr(objectKey),
	}, func(o *oss.PresignOptions) {
		o.Expires = 1 * time.Hour // 设置有效期，V4 签名最长支持 7 天
	})
	if err != nil {
		log.Fatalf("生成预签名 URL 失败: %v", err)
	}

	fmt.Printf("🔗 临时下载链接: %s\n", presignResult.URL)
	fmt.Printf("⏰ 过期时间: %s\n", presignResult.Expiration.Format("2006-01-02 15:04:05"))
}
```

---

### 🔑 关键说明

| 模块 | 说明 |
|------|------|
| **双 Endpoint 设计** | ECS 上传使用 `-internal` 内网域名（免流量费、低延迟）；生成预签名 URL 使用外网域名，确保终端用户从公网可正常访问 |
| **凭证获取** | 示例使用环境变量。生产环境强烈建议 ECS 绑定 **RAM 角色**，通过 `credentials.NewEcsRamRoleCredentialsProvider("role-name")` 自动获取临时凭证，避免硬编码 AK/SK |
| **签名版本** | SDK V2 默认使用 V4 签名，预签名 URL 有效期**最长 7 天** |
| **错误处理** | 实际业务中建议对 `err` 进行重试或降级处理，避免单次网络抖动导致服务中断 |

---

### 🛠 环境准备

1. **安装 SDK**
   ```bash
   go mod init oss-demo
   go get github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss
   ```

2. **配置访问凭证**（二选一）
   ```bash
   # 方式 A：环境变量（开发测试）
   export OSS_ACCESS_KEY_ID="your-access-key-id"
   export OSS_ACCESS_KEY_SECRET="your-access-key-secret"

   # 方式 B：ECS RAM 角色（生产推荐）
   # 1. 在 RAM 控制台创建角色并授予 oss:PutObject / oss:GetObject 权限
   # 2. 在 ECS 控制台为实例绑定该角色
   # 3. 代码中替换凭证提供者：
   # credentials.NewEcsRamRoleCredentialsProvider("your-ecs-ram-role-name")
   ```

3. **运行测试**
   ```bash
   go run main.go
   ```

---

### 📖 参考文档
- [OSS Go SDK V2 使用预签名 URL 下载](https://help.aliyun.com/document_detail/2859275.html)
- [OSS Go SDK V2 使用预签名 URL 上传](https://help.aliyun.com/document_detail/2859330.html)
- [ECS 实例 RAM 角色绑定指南](https://help.aliyun.com/document_detail/2834342.html)

如需集成到 Web 框架（如 Gin/Echo）或需要处理图片压缩/水印等预处理逻辑，可提供具体场景，我将补充对应代码。