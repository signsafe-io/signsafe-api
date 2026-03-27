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

const contractsBucket = "contracts"

// Client wraps an AWS S3-compatible client for SeaweedFS.
type Client struct {
	s3     *s3.Client
	bucket string
}

// NewClient creates a new S3-compatible storage client.
func NewClient(endpoint, accessKey, secretKey string) *Client {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		},
	)

	cfg := aws.Config{
		Region:                      "us-east-1",
		EndpointResolverWithOptions: resolver,
		Credentials:                 credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}

	return &Client{
		s3:     s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }),
		bucket: contractsBucket,
	}
}

// Put uploads a file to SeaweedFS under the given key.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("storage.Put: %w", err)
	}
	return nil
}

// Get downloads a file from SeaweedFS.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("storage.Get: %w", err)
	}
	return out.Body, nil
}

// Delete removes a file from SeaweedFS.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage.Delete: %w", err)
	}
	return nil
}

// PresignedGetURL generates a presigned URL valid for the given duration.
func (c *Client) PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presigner := s3.NewPresignClient(c.s3)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage.PresignedGetURL: %w", err)
	}
	return req.URL, nil
}
