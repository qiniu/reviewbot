package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Storage struct {
	s3     *s3.S3
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

func NewS3Storage(credential []byte) (Storage, error) {
	s3Creds := &s3Credentials{}
	if err := json.Unmarshal(credential, s3Creds); err != nil {
		return nil, fmt.Errorf("error getting S3 credentials from JSON: %w", err)
	}
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(s3Creds.Region),
		Endpoint:         aws.String(s3Creds.Endpoint),
		S3ForcePathStyle: aws.Bool(s3Creds.S3ForcePathStyle),
		Credentials:      credentials.NewStaticCredentials(s3Creds.AccessKey, s3Creds.SecretKey, ""),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	svc := s3.New(sess)

	return &S3Storage{
		s3:     svc,
		bucket: s3Creds.Bucket,
	}, nil
}

func (s *S3Storage) Write(ctx context.Context, key string, content []byte) error {
	reader := bytes.NewReader(content)
	objectKey := filepath.Join(key, DefaultLogName)
	_, err := s.s3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   aws.ReadSeekCloser(reader),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	return nil
}

func (s *S3Storage) Read(ctx context.Context, key string) ([]byte, error) {
	objectKey := filepath.Join(key, DefaultLogName)
	result, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file from s3 bucket: %w", err)
	}
	output, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from  s3 result body: %w", err)
	}
	return output, nil
}
