/*
 * minio_service.go
 * 功能：封装 MinIO 对象存储操作，为上层提供预签名 URL、对象上传下载等能力
 *      预签名函数显式接受 bucket 参数，避免视频/封面分桶时上传与校验对不上
 * 时间戳：2026-05-01
 */

package minio

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/shermon/SesameSauce/config"
	"github.com/spf13/viper"
)

// GetVideoBucket 返回存储原始视频与转码产物的私有 bucket 名，未配置时回落默认值
func GetVideoBucket() string {
	bucket := viper.GetString("minio.bucket")
	if bucket == "" {
		bucket = "sesame-sauce"
	}
	return bucket
}

// GetPublicBucket 返回存储公开访问内容（封面、m3u8 转码产物）的公共 bucket 名，未配置时回落默认值
// 该 bucket 在 MinIO 侧需配置为 anonymous read，便于浏览器/hls.js 直连访问 ts 切片
func GetPublicBucket() string {
	bucket := viper.GetString("minio.public_bucket")
	if bucket == "" {
		bucket = "sesame-sauce-public"
	}
	return bucket
}

// GeneratePresignedPutURL 在指定 bucket 下生成用于上传的预签名 PUT URL
// 若配置了 external_endpoint，返回的 URL 会替换为外部可访问地址
func GeneratePresignedPutURL(bucket, objectKey string, expires time.Duration) (string, error) {
	url, err := config.MinioClient.PresignedPutObject(context.Background(), bucket, objectKey, expires)
	if err != nil {
		return "", err
	}
	return rewriteMinIOURL(url.String()), nil
}

// GeneratePresignedGetURL 在指定 bucket 下生成用于下载/播放的预签名 GET URL
// 若配置了 external_endpoint，返回的 URL 会替换为外部可访问地址
func GeneratePresignedGetURL(bucket, objectKey string, expires time.Duration) (string, error) {
	url, err := config.MinioClient.PresignedGetObject(context.Background(), bucket, objectKey, expires, nil)
	if err != nil {
		return "", err
	}
	return rewriteMinIOURL(url.String()), nil
}

// StatObject 检查对象是否存在
func StatObject(ctx context.Context, bucket, objectKey string) error {
	_, err := config.MinioClient.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	return err
}

// DownloadObject 将对象下载到本地路径
func DownloadObject(ctx context.Context, bucket, objectKey, localPath string) error {
	return config.MinioClient.FGetObject(ctx, bucket, objectKey, localPath, minio.GetObjectOptions{})
}

// UploadObject 将本地文件上传到 MinIO
func UploadObject(ctx context.Context, bucket, objectKey, localPath, contentType string) error {
	_, err := config.MinioClient.FPutObject(ctx, bucket, objectKey, localPath, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// BuildVideoBaseURL 根据配置拼接基础访问 URL
// 若配置了 external_endpoint，返回 scheme://external_endpoint/bucket（用于生产环境 Nginx 代理）
// 否则返回 scheme://endpoint/bucket（用于开发环境直连 MinIO）
func BuildVideoBaseURL(bucket string) string {
	useSSL := viper.GetBool("minio.use_ssl")
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	endpoint := viper.GetString("minio.external_endpoint")
	if endpoint == "" {
		endpoint = viper.GetString("minio.endpoint")
	}
	return fmt.Sprintf("%s://%s/%s", scheme, endpoint, bucket)
}

// rewriteMinIOURL 将预签名 URL 中的内部 endpoint 替换为外部可访问地址
// Nginx 代理时需保持 Host: localhost:9000，以确保 MinIO 签名验证通过
func rewriteMinIOURL(rawURL string) string {
	externalEndpoint := viper.GetString("minio.external_endpoint")
	if externalEndpoint == "" {
		return rawURL
	}
	endpoint := viper.GetString("minio.endpoint")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	useSSL := viper.GetBool("minio.use_ssl")
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	oldPrefix := fmt.Sprintf("%s://%s", scheme, endpoint)
	newPrefix := fmt.Sprintf("%s://%s", scheme, externalEndpoint)
	return strings.Replace(rawURL, oldPrefix, newPrefix, 1)
}

// DeleteObject 删除指定对象
func DeleteObject(ctx context.Context, bucket, objectKey string) error {
	return config.MinioClient.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

// ListObjects 列举指定前缀的对象
func ListObjects(ctx context.Context, bucket, prefix string) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo
	for obj := range config.MinioClient.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}
