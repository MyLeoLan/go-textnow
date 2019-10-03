package redis

import (
	"log"

	"github.com/OmarElGabry/go-textnow/internal/pkg/config"
	"github.com/go-redis/redis"
)

// Cache Redis
type Cache struct {
	*redis.Client
	ErrNotExists error
}

// NewCache creates and returns connection to Redis
func NewCache() (*Cache, error) {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config("REDIS_ADDR"),
		Password: config("REDIS_PASSWORD"),
		DB:       0, // use default DB
	})

	_, err = client.Ping().Result()
	if err != nil {
		return nil, err
	}

	return &Cache{client, redis.Nil}, nil
}
