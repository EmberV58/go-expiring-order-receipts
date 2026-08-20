package signeddownload

import (
	"context"

	"example.com/private-order-downloads/infrai"
)

type InfraiStorage struct{ Client *infrai.Client }

func (s InfraiStorage) HeadObject(ctx context.Context, bucket, key string) (HeadResult, error) {
	result, err := s.Client.HeadObject(ctx, bucket, key)
	return HeadResult{Found: result.Found}, err
}

func (s InfraiStorage) PresignGet(ctx context.Context, bucket, key string, ttl int, disposition, idempotencyKey string) (LinkResult, error) {
	result, err := s.Client.PresignGet(ctx, bucket, key, infrai.PresignGetRequest{
		Op: "get", ExpiresSeconds: ttl, ResponseDisposition: disposition, IdempotencyKey: idempotencyKey,
	})
	return LinkResult{URL: result.URL}, err
}
