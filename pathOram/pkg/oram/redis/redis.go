package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
	"pathOram/pkg/oram/bucket"
	"pathOram/pkg/oram/crypto"
)

type RedisClient struct {
	Client        *redis.Client
	EncryptionKey []byte
	Ctx           context.Context
}

func NewRedisClient(redisAddr string, encryptionKey []byte) (*RedisClient, error) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &RedisClient{
		Client:        client,
		EncryptionKey: encryptionKey,
		Ctx:           ctx,
	}, nil
}

func (r *RedisClient) WriteBucketToDb(index int, bucket bucket.Bucket) error {
	data, err := json.Marshal(bucket)
	if err != nil {
		return err
	}

	encryptedData, err := crypto.Encrypt(data, r.EncryptionKey)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("bucket:%d", index)
	err = r.Client.Set(r.Ctx, key, encryptedData, 0).Err()
	return err
}

func (r *RedisClient) ReadBucketFromDb(index int) (bucket.Bucket, error) {
	key := fmt.Sprintf("bucket:%d", index)
	data, err := r.Client.Get(r.Ctx, key).Bytes()
	if err != nil {
		return bucket.Bucket{}, err
	}

	decryptedData, err := crypto.Decrypt(data, r.EncryptionKey)
	if err != nil {
		return bucket.Bucket{}, err
	}

	var bucket1 bucket.Bucket
	err = json.Unmarshal(decryptedData, &bucket1)
	if err != nil {
		return bucket.Bucket{}, err
	}

	return bucket1, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}
