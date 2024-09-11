package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var ErrObjectNotFound = errors.New("object not found")

type S3Storage struct {
	s3     *s3.Client
	bucket string
}
type s3Credentials struct {
	Region           string `json:"region"`
	Endpoint         string `json:"endpoint"`
	Insecure         bool   `json:"insecure"`
	S3ForcePathStyle bool   `json:"s3ForcePathStyle"`
	AccessKey        string `json:"accessKey"`
	SecretKey        string `json:"secretKey"`
	Bucket           string `json:"bucket"`
}

func NewS3Storage(credFilePath string) (Storage, error) {
	credential, err := os.ReadFile(credFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open S3CredentialsFile: %w", err)
	}
	s3Creds := &s3Credentials{}
	if err := json.Unmarshal(credential, s3Creds); err != nil {
		return nil, fmt.Errorf("error getting S3 credentials from JSON: %w", err)
	}

	svc := s3.NewFromConfig(aws.Config{}, func(o *s3.Options) {
		o.Credentials = credentials.NewStaticCredentialsProvider(s3Creds.AccessKey, s3Creds.SecretKey, "")
		o.Region = s3Creds.Region
		o.BaseEndpoint = &s3Creds.Endpoint
		o.UsePathStyle = s3Creds.S3ForcePathStyle
	})
	return &S3Storage{
		s3:     svc,
		bucket: s3Creds.Bucket,
	}, nil
}

func (s *S3Storage) Write(ctx context.Context, key string, content []byte) error {
	reader := bytes.NewReader(content)
	objectKey := filepath.Join(key, DefaultLogName)
	_, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   reader,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	return nil
}

func (s *S3Storage) Read(ctx context.Context, key string) ([]byte, error) {
	objectKey := filepath.Join(key, DefaultLogName)
	result, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" {
				return nil, ErrObjectNotFound
			}
		}
		return nil, fmt.Errorf("failed to download file from s3 bucket: %w", err)
	}
	defer result.Body.Close()

	output, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from s3 result body: %w", err)
	}
	return output, nil
}
