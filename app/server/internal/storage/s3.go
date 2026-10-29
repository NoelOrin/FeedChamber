package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Provider 兼容所有 S3 API 的对象存储（AWS S3 / MinIO / Cloudflare R2 / 阿里云 OSS 等）。
type S3Provider struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	customDomain  string
	region        string
}

func NewS3Provider(cfg Config) (*S3Provider, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.AccessKeySecret, ""),
	}

	clientOpts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // MinIO 等兼容存储需要 path-style
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)
	presignClient := s3.NewPresignClient(client)

	return &S3Provider{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		customDomain:  cfg.CustomDomain,
		region:        cfg.Region,
	}, nil
}

func (p *S3Provider) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 put object: %w", err)
	}
	return nil
}

func (p *S3Provider) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, nil
}

func (p *S3Provider) Delete(ctx context.Context, key string) error {
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}

func (p *S3Provider) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	items := make([]ObjectInfo, 0)
	paginator := s3.NewListObjectsV2Paginator(p.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(p.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			item := ObjectInfo{Key: *obj.Key}
			if obj.Size != nil {
				item.Size = *obj.Size
			}
			if obj.LastModified != nil {
				item.ModifiedAt = *obj.LastModified
			}
			items = append(items, item)
		}
	}
	return items, nil
}

func (p *S3Provider) PresignPostURL(ctx context.Context, key string, contentType string, maxSize int64, expires time.Duration) (*PresignedPost, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	postObj, err := p.presignClient.PresignPostObject(ctx, input, func(o *s3.PresignPostOptions) {
		o.Expires = expires
		if maxSize > 0 {
			o.Conditions = append(o.Conditions, []any{"content-length-range", 0, maxSize})
		}
	})
	if err != nil {
		return nil, fmt.Errorf("s3 presign post: %w", err)
	}

	fields := make(map[string]string, len(postObj.Values))
	for k, v := range postObj.Values {
		fields[k] = v
	}

	return &PresignedPost{
		URL:    postObj.URL,
		Fields: fields,
	}, nil
}

func (p *S3Provider) PublicURL(key string) string {
	if p.customDomain != "" {
		return p.customDomain + "/" + EncodeKey(key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", p.bucket, p.region, EncodeKey(key))
}
