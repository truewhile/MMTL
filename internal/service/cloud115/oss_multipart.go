// 阿里云 OSS multipart 分片上传（用于 115 元数据上传直传）。
// 使用 115 下发的临时 STS 凭证，将本地文件分片上传到 OSS，并经 complete 回调
// 通知 115 完成落盘。参考 QMediaSync 的 OSSMultipartUploader 实现。
package cloud115

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

const (
	defaultMultipartPartSize int64 = 32 * 1024 * 1024
	multipartPartAlign       int64 = 1024 * 1024
	maxMultipartParts        int64 = 9999
	maxMultipartPartSize     int64 = 5 * 1024 * 1024 * 1024
)

type ossMultipartClient interface {
	InitiateMultipartUpload(context.Context, *oss.InitiateMultipartUploadRequest, ...func(*oss.Options)) (*oss.InitiateMultipartUploadResult, error)
	UploadPart(context.Context, *oss.UploadPartRequest, ...func(*oss.Options)) (*oss.UploadPartResult, error)
	ListParts(context.Context, *oss.ListPartsRequest, ...func(*oss.Options)) (*oss.ListPartsResult, error)
	CompleteMultipartUpload(context.Context, *oss.CompleteMultipartUploadRequest, ...func(*oss.Options)) (*oss.CompleteMultipartUploadResult, error)
	AbortMultipartUpload(context.Context, *oss.AbortMultipartUploadRequest, ...func(*oss.Options)) (*oss.AbortMultipartUploadResult, error)
}

// OSSMultipartUploader 封装 OSS multipart 上传。
type OSSMultipartUploader struct {
	client ossMultipartClient
}

// OSSMultipartUploadInput 是 multipart 上传输入。
type OSSMultipartUploadInput struct {
	Bucket        string
	Object        string
	Callback      string
	CallbackVar   string
	FilePath      string
	FileSize      int64
	UploadId      string
	PartSize      int64
	PartRetryMax  int
	refreshClient func(context.Context) (ossMultipartClient, error)
}

// OSSMultipartUploadResult 是 multipart 上传后的结果。
type OSSMultipartUploadResult struct {
	CallbackResult map[string]any
	UploadId       string
	PartSize       int64
	TotalParts     int
	UploadedBytes  int64
	UploadedParts  int
}

// CalculateMultipartPartSize 计算 OSS multipart 分片大小与分片数量。
func CalculateMultipartPartSize(fileSize int64) (int64, int, error) {
	if fileSize < 0 {
		return 0, 0, fmt.Errorf("文件大小不能为负数：%d", fileSize)
	}
	partSize := defaultMultipartPartSize
	minPartSize := ceilDiv(fileSize, maxMultipartParts)
	if minPartSize > partSize {
		partSize = roundUp(minPartSize, multipartPartAlign)
	}
	if partSize > maxMultipartPartSize {
		return 0, 0, fmt.Errorf("文件过大，所需分片大小 %d 超过 OSS 上限 %d", partSize, maxMultipartPartSize)
	}
	totalParts := int(ceilDiv(fileSize, partSize))
	if totalParts == 0 {
		totalParts = 1
	}
	if int64(totalParts) > maxMultipartParts {
		return 0, 0, fmt.Errorf("分片数量 %d 超过上限 %d", totalParts, maxMultipartParts)
	}
	return partSize, totalParts, nil
}

// NewOSSMultipartUploader 创建 OSS multipart 上传器。
func NewOSSMultipartUploader(endpoint, accessKeyId, accessKeySecret, securityToken string) *OSSMultipartUploader {
	return &OSSMultipartUploader{client: newOSSMultipartClient(endpoint, accessKeyId, accessKeySecret, securityToken)}
}

func newOSSMultipartClient(endpoint, accessKeyId, accessKeySecret, securityToken string) ossMultipartClient {
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, securityToken)).
		WithRegion("cn-shenzhen").
		WithEndpoint(endpoint)
	return oss.NewClient(cfg)
}

// UploadFile 上传文件并完成 OSS multipart，返回 complete callback 结果。
func (u *OSSMultipartUploader) UploadFile(ctx context.Context, input OSSMultipartUploadInput) (map[string]any, error) {
	result, err := u.UploadFileWithResult(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.CallbackResult, nil
}

// UploadFileWithResult 上传文件并返回 multipart 结果。
func (u *OSSMultipartUploader) UploadFileWithResult(ctx context.Context, input OSSMultipartUploadInput) (OSSMultipartUploadResult, error) {
	if input.PartRetryMax <= 0 {
		input.PartRetryMax = 3
	}
	partSize := input.PartSize
	totalParts := 0
	var err error
	if partSize <= 0 {
		partSize, totalParts, err = CalculateMultipartPartSize(input.FileSize)
		if err != nil {
			return OSSMultipartUploadResult{}, err
		}
	} else {
		totalParts = int(ceilDiv(input.FileSize, partSize))
	}

	uploadId := input.UploadId
	if uploadId == "" {
		initResult, err := u.client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{
			Bucket: oss.Ptr(input.Bucket),
			Key:    oss.Ptr(input.Object),
			RequestCommon: oss.RequestCommon{
				Parameters: map[string]string{"sequential": "1"},
			},
		})
		if err != nil {
			return OSSMultipartUploadResult{}, fmt.Errorf("初始化 OSS multipart 失败：%w", err)
		}
		if initResult.UploadId == nil || *initResult.UploadId == "" {
			return OSSMultipartUploadResult{}, fmt.Errorf("初始化 OSS multipart 返回空 upload_id")
		}
		uploadId = *initResult.UploadId
	}

	existingPartMap := make(map[int32]int64)
	existingParts, err := u.ListUploadedParts(ctx, input.Bucket, input.Object, uploadId)
	if err == nil {
		for _, part := range existingParts {
			existingPartMap[part.PartNumber] = part.Size
		}
	}

	file, err := os.Open(input.FilePath)
	if err != nil {
		return OSSMultipartUploadResult{}, fmt.Errorf("打开待上传文件失败：%w", err)
	}
	defer file.Close()

	var uploadedBytes int64
	uploadedParts := 0
	completeParts := make([]oss.UploadPart, 0, totalParts)
	for partNumber := 1; partNumber <= totalParts; partNumber++ {
		offset := int64(partNumber-1) * partSize
		length := minInt64(partSize, input.FileSize-offset)
		if length < 0 {
			length = 0
		}
		if existingSize, ok := existingPartMap[int32(partNumber)]; ok && existingSize == length {
			uploadedBytes += length
			uploadedParts++
		}
		etag, err := u.uploadPartWithRetry(ctx, input, uploadId, int32(partNumber), file, offset, length)
		if err != nil {
			return OSSMultipartUploadResult{}, err
		}
		uploadedBytes += length
		uploadedParts++
		completeParts = append(completeParts, oss.UploadPart{
			PartNumber: int32(partNumber),
			ETag:       oss.Ptr(etag),
		})
	}
	sort.Slice(completeParts, func(i, j int) bool {
		return completeParts[i].PartNumber < completeParts[j].PartNumber
	})

	// 115 下发的 callback / callback_var 是 JSON 字符串，而 OSS CompleteMultipartUpload
	// 要求 callback 参数为 Base64 编码后的 JSON，否则报 "The callback configuration is
	// not base64 encoded"。这里把两者转为 Base64 后再提交（参考 QMediaSync 的
	// BuildOSSCallbackHeaders）。
	cb := input.Callback
	cbVar := input.CallbackVar
	if cb == "" {
		return OSSMultipartUploadResult{}, errors.New("OSS callback 为空")
	}
	if !json.Valid([]byte(cb)) {
		return OSSMultipartUploadResult{}, errors.New("解析 callback 失败：不是合法 JSON")
	}
	if cbVar == "" {
		cbVar = "{}"
	}
	if !json.Valid([]byte(cbVar)) {
		return OSSMultipartUploadResult{}, errors.New("解析 callback_var 失败：不是合法 JSON")
	}
	completeResult, err := u.client.CompleteMultipartUpload(ctx, &oss.CompleteMultipartUploadRequest{
		Bucket:   oss.Ptr(input.Bucket),
		Key:      oss.Ptr(input.Object),
		UploadId: oss.Ptr(uploadId),
		CompleteMultipartUpload: &oss.CompleteMultipartUpload{
			Parts: completeParts,
		},
		Callback:    oss.Ptr(base64.StdEncoding.EncodeToString([]byte(cb))),
		CallbackVar: oss.Ptr(base64.StdEncoding.EncodeToString([]byte(cbVar))),
	})
	if err != nil {
		return OSSMultipartUploadResult{}, fmt.Errorf("完成 OSS multipart 失败：%w", err)
	}
	return OSSMultipartUploadResult{
		CallbackResult: completeResult.CallbackResult,
		UploadId:       uploadId,
		PartSize:       partSize,
		TotalParts:     totalParts,
		UploadedBytes:  uploadedBytes,
		UploadedParts:  uploadedParts,
	}, nil
}

// ListUploadedParts 查询 OSS 已上传分片。
func (u *OSSMultipartUploader) ListUploadedParts(ctx context.Context, bucket, object, uploadId string) ([]struct {
	PartNumber int32
	Size       int64
}, error) {
	parts := []struct {
		PartNumber int32
		Size       int64
	}{}
	result, err := u.client.ListParts(ctx, &oss.ListPartsRequest{
		Bucket:   oss.Ptr(bucket),
		Key:      oss.Ptr(object),
		UploadId: oss.Ptr(uploadId),
		MaxParts: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("查询 OSS 已上传分片失败：%w", err)
	}
	for _, part := range result.Parts {
		parts = append(parts, struct {
			PartNumber int32
			Size       int64
		}{PartNumber: part.PartNumber, Size: part.Size})
	}
	return parts, nil
}

func (u *OSSMultipartUploader) uploadPartWithRetry(
	ctx context.Context,
	input OSSMultipartUploadInput,
	uploadId string,
	partNumber int32,
	file *os.File,
	offset, length int64,
) (string, error) {
	var lastErr error
	for attempt := 0; attempt < input.PartRetryMax; attempt++ {
		reader := io.NewSectionReader(file, offset, length)
		result, err := u.client.UploadPart(ctx, &oss.UploadPartRequest{
			Bucket:        oss.Ptr(input.Bucket),
			Key:           oss.Ptr(input.Object),
			PartNumber:    partNumber,
			UploadId:      oss.Ptr(uploadId),
			Body:          reader,
			ContentLength: oss.Ptr(length),
		})
		if err == nil {
			if result.ETag == nil || *result.ETag == "" {
				return "", fmt.Errorf("OSS part %d 返回空 ETag", partNumber)
			}
			return *result.ETag, nil
		}
		lastErr = err
		if attempt < input.PartRetryMax-1 && input.refreshClient != nil {
			refreshed, refreshErr := input.refreshClient(ctx)
			if refreshErr != nil {
				lastErr = refreshErr
				continue
			}
			u.client = refreshed
		}
	}
	return "", fmt.Errorf("上传 OSS part %d 失败：%w", partNumber, lastErr)
}

// ParseCompleteCallbackResult 校验并解析 OSS complete 后的 115 callback 结果。
func ParseCompleteCallbackResult(result map[string]any) (UploadCompleteResult, error) {
	if result == nil {
		return UploadCompleteResult{}, errors.New("OSS complete callback 结果为空")
	}
	if state, ok := result["state"].(bool); ok && !state {
		return UploadCompleteResult{}, fmt.Errorf("115 callback 返回失败：%s", anyToString(result["message"]))
	}
	if message := anyToString(result["message"]); message != "" {
		return UploadCompleteResult{}, fmt.Errorf("115 callback 返回错误：%s", message)
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		return UploadCompleteResult{}, errors.New("115 callback 缺少 data")
	}
	complete := UploadCompleteResult{
		FileId:   anyToString(data["file_id"]),
		PickCode: anyToString(data["pick_code"]),
		ParentId: anyToString(data["parent_id"]),
		Sha1:     anyToString(data["sha1"]),
		Size:     anyToInt64(data["size"]),
		Mtime:    anyToInt64(data["mtime"]),
	}
	if complete.FileId == "" || complete.PickCode == "" {
		return UploadCompleteResult{}, errors.New("115 callback 缺少 file_id/pick_code")
	}
	return complete, nil
}

func ceilDiv(n, d int64) int64 {
	if d <= 0 {
		return 0
	}
	if n <= 0 {
		return 0
	}
	return (n + d - 1) / d
}

func roundUp(n, align int64) int64 {
	if align <= 0 {
		return n
	}
	return ceilDiv(n, align) * align
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func anyToInt64(v any) int64 {
	switch t := v.(type) {
	case string:
		var n int64
		fmt.Sscanf(t, "%d", &n)
		return n
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
