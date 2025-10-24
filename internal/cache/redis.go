package cache

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewCache(redisURL, password string, ttl int) (*Cache, error) {
	// Parse Redis URL if it contains redis:// or rediss://
	addr := redisURL
	pass := password

	if strings.HasPrefix(redisURL, "redis://") || strings.HasPrefix(redisURL, "rediss://") {
		parsedURL, err := url.Parse(redisURL)
		if err != nil {
			return nil, err
		}

		// Extract host:port
		addr = parsedURL.Host

		// Extract password if present in URL
		if parsedURL.User != nil {
			if p, ok := parsedURL.User.Password(); ok {
				pass = p
			}
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Cache{
		client: client,
		ttl:    time.Duration(ttl) * time.Second,
	}, nil
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key).Bytes()
}

func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	return c.client.Set(ctx, key, value, c.ttl).Err()
}

func (c *Cache) Close() error {
	return c.client.Close()
}
