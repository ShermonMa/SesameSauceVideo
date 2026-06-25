/*
 * minio.go
 * 功能：MinIO 客户端初始化与 bucket 自动创建
 * 时间戳：2026-04-22
 */

package config

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
)

// MinioClient 是全局 MinIO 客户端实例
var MinioClient *minio.Client

// InitMinio 根据 config.yaml 初始化 MinIO 客户端，并确保 bucket 存在
func InitMinio() error {
	endpoint := viper.GetString("minio.endpoint")
	accessKey := viper.GetString("minio.access_key_id")
	secretKey := viper.GetString("minio.secret_access_key")
	useSSL := viper.GetBool("minio.use_ssl")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("初始化 MinIO 客户端失败: %w", err)
	}

	MinioClient = client

	ctx := context.Background()

	// 私有 bucket：存视频
	bucket := viper.GetString("minio.bucket")
	if bucket == "" {
		bucket = "sesame-sauce"
	}
	if err := ensureBucket(ctx, client, bucket, false); err != nil {
		return err
	}

	// 公有 bucket：存封面，允许匿名读取
	publicBucket := viper.GetString("minio.public_bucket")
	if publicBucket == "" {
		publicBucket = "sesame-sauce-public"
	}
	if err := ensureBucket(ctx, client, publicBucket, true); err != nil {
		return err
	}

	log.Printf("MinIO 客户端初始化完成, private bucket: %s, public bucket: %s", bucket, publicBucket)
	return nil
}

// ensureBucket 检查并创建 bucket；isPublic=true 时附加匿名只读 policy
func ensureBucket(ctx context.Context, client *minio.Client, bucket string, isPublic bool) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("检查 bucket %s 存在性失败: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("创建 bucket %s 失败: %w", bucket, err)
		}
		log.Printf("MinIO bucket 已创建: %s", bucket)
	}
	if isPublic {
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
		if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
			return fmt.Errorf("设置 bucket %s 公开策略失败: %w", bucket, err)
		}
		log.Printf("MinIO bucket %s 已设为公开读取", bucket)
	}
	return nil
}
