package redisStore

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/redis/go-redis/v9"
)

var (
	instances = make(map[int]*Store)
	mu        sync.RWMutex
	logger    *logger_i.Logger
	once      sync.Once
)

type Store struct {
	client *redis.Client
	Type   int
}

func GetRedisStore(ctx context.Context, DBType int) *Store {

	mu.RLock()
	instance, exists := instances[DBType]
	mu.RUnlock()

	if exists {
		return instance
	}

	mu.Lock()
	defer mu.Unlock()

	if instance, exists = instances[DBType]; exists {
		return instance
	}
	return createNewStore(ctx, DBType)

}

func initLogger(dbtype int) {
	if logger == nil {
		logger = logger_i.NewLogger("Redis Store: " + string(rune(dbtype)))
	}
}

func closeRedisStores(ctx context.Context) {
	<-ctx.Done()
	logger.Info("Closing Redis Stores")
	mu.Lock()
	defer mu.Unlock()
	for _, store := range instances {
		err := store.client.Close()
		if err != nil {
			logger.Error("Error closing redis client", "error", err)
		}
	}
	logger.Info("Redis Store Closed successfully")
}

func createNewStore(ctx context.Context, dbType int) *Store {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = config.RedisAddr
	}
	newClient := redis.NewClient(&redis.Options{
		Addr:                  addr,
		Password:              config.RedisPassword,
		DB:                    dbType,
		ContextTimeoutEnabled: true,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
	})

	initLogger(dbType)

	if newClient == nil {
		logger.Error("could not connect to redis")
		return nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := newClient.Ping(pingCtx).Err(); err != nil {
		logger.Error("Redis is offline: ", err.Error())
		return nil
	}

	logger.Info("Redis Router init successfully")

	newStore := &Store{
		client: newClient,
		Type:   dbType,
	}

	instances[dbType] = newStore
	once.Do(func() {
		go closeRedisStores(ctx)
	})
	return newStore

}

// Only in a _test.go file or behind a build tag
func NewTestStore(client *redis.Client) *Store {
	return &Store{
		client: client,
		// ... other fields
	}
}
