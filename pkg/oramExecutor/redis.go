package oramexecutor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

/*
MGET is faster than GET only because its 1RTT + redis processing time, with GET, there is 1RTT for each GET request
*/
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

func (r *RedisClient) FlushDB() error {
	return r.Client.FlushDB(r.Ctx).Err()
}

func (r *RedisClient) FlushData() {
	ctx := context.Background()
	r.Client.FlushAll(ctx)
}

// type BucketRequest struct {
//     bucketId int
//     bucket   bucket.Bucket
// }

func (r *RedisClient) WriteBucketsToDb(requests []BucketRequest) error {
	// Prepare data for MSET
	msetArgs := make([]interface{}, 0, len(requests)*2)

	for _, req := range requests {
		data, err := json.Marshal(req.Bucket)
		if err != nil {
			return err
		}

		encryptedData, err := Encrypt(data, r.EncryptionKey)
		if err != nil {
			return err
		}

		key := fmt.Sprintf("bucket:%d", req.BucketId)
		msetArgs = append(msetArgs, key, encryptedData)
	}

	// Use MSET to set multiple key-value pairs at once
	_, err := r.Client.MSet(r.Ctx, msetArgs...).Result() // Use MSet method directly
	return err
}

func (r *RedisClient) ReadBucketsFromDb(indices map[int]struct{}) (map[int]Bucket, error) {
	// Convert map keys to slice for MGET
	keys := make([]string, 0, len(indices))
	for index := range indices {
		keys = append(keys, fmt.Sprintf("bucket:%d", index))
	}

	// Perform MGET operation
	data, err := r.Client.MGet(r.Ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	// Initialize a map to store the retrieved buckets
	buckets := make(map[int]Bucket, len(indices))

	// Iterate over the retrieved data
	for i, raw := range data {
		if raw == nil {
			continue
		}
		encryptedData, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected data type: %T", raw)
		}

		// Decrypt the data
		decryptedData, err := Decrypt([]byte(encryptedData), r.EncryptionKey)
		if err != nil {
			return nil, err
		}

		// Unmarshal the decrypted data into a bucket
		var bucket1 Bucket
		err = json.Unmarshal(decryptedData, &bucket1)
		if err != nil {
			return nil, err
		}

		// Map the bucket to the corresponding index
		var index int
		fmt.Sscanf(keys[i], "bucket:%d", &index)
		buckets[index] = bucket1
	}

	return buckets, nil
}

func (r *RedisClient) WriteBucketToDb(index int, bucket Bucket) error {
	data, err := json.Marshal(bucket)
	if err != nil {
		return err
	}

	encryptedData, err := Encrypt(data, r.EncryptionKey)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("bucket:%d", index)
	err = r.Client.Set(r.Ctx, key, encryptedData, 0).Err()
	//fmt.Println("writing current request to redis; redis side ")
	return err
}

func (r *RedisClient) ReadBucketFromDb(index int) (Bucket, error) {
	key := fmt.Sprintf("bucket:%d", index)
	data, err := r.Client.Get(r.Ctx, key).Bytes()
	if err != nil {
		return Bucket{}, err
	}

	decryptedData, err := Decrypt(data, r.EncryptionKey)
	if err != nil {
		return Bucket{}, err
	}

	var bucket1 Bucket
	err = json.Unmarshal(decryptedData, &bucket1)
	if err != nil {
		return Bucket{}, err
	}

	return bucket1, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}
