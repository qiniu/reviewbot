package storage

import "context"

type S3Storage struct {
	credential string
}

func NewS3Storage(credential string) (Storage, error) {
	return &S3Storage{
		credential: credential,
	}, nil
}

func (s *S3Storage) Write(ctx context.Context, key string, content []byte) error {
	return nil
}

func (s *S3Storage) Read(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
