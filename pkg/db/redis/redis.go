package redis

import (
	"context"
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func InitRedisConfig() *RedisClient {
	// redis config
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisHost = redisHost + ":" + redisPort
	log.Info("Redis host to connect: ", redisHost)

	redisPass := os.Getenv("REDIS_PASS")
	if redisHost == "" {
		redisPass = ""
	}

	return NewRedisClient(redisHost, redisPass, 0)
}

func NewRedisClient(addr string, password string, db int) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // use password set
		DB:       db,       // use DB
		Protocol: 3,        // specify 2 for RESP 2 or 3 for RESP 3
	})
	return &RedisClient{client}
}

func (r *RedisClient) SetKey(ctx context.Context, key, value string, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) GetKey(ctx context.Context, key string) (string, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return result, nil
}

func (r *RedisClient) SetKeyExpiration(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}
