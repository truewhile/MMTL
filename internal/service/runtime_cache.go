package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/config"
)

type RuntimeCacheService struct {
	log    *zap.Logger
	client *redis.Client
	prefix string

	mu     sync.RWMutex
	memory map[string]runtimeCacheItem
	obj    map[string]runtimeObjectItem
	limit  int
}

type runtimeCacheItem struct {
	raw       []byte
	expiresAt time.Time
}

// runtimeObjectItem 直存 Go 对象，跳过 JSON 编解码。热点路径（整库行、
// 分组结果）每次请求都要完整反序列化，字节缓存避免了 SQL 却没避免解码；
// 对象缓存命中时零解码零分配。存储的值视为不可变：读取方如需修改必须
// 自行浅拷贝。仅进程内生效（Redis 只支持字节），多实例部署退化为各实例
// 独立缓存，与现有内存 L1 语义一致。
type runtimeObjectItem struct {
	value     any
	expiresAt time.Time
}

func NewRuntimeCacheService(cfg *config.Config, log *zap.Logger) *RuntimeCacheService {
	c := &RuntimeCacheService{log: log, memory: map[string]runtimeCacheItem{}, obj: map[string]runtimeObjectItem{}, limit: 2048}
	if cfg == nil {
		return c
	}
	c.prefix = strings.Trim(strings.TrimSpace(cfg.Cache.RedisPrefix), ":")
	if c.prefix == "" {
		c.prefix = "mebox"
	}
	rawURL := strings.TrimSpace(cfg.Cache.RedisURL)
	if rawURL == "" {
		return c
	}
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		if log != nil {
			log.Warn("redis cache disabled: invalid redis url", zap.Error(err))
		}
		return c
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		if log != nil {
			log.Warn("redis cache unavailable; using in-process cache", zap.Error(err))
		}
		_ = client.Close()
		return c
	}
	c.client = client
	if log != nil {
		log.Info("redis runtime cache enabled with in-process L1", zap.String("addr", opts.Addr), zap.String("prefix", c.prefix))
	}
	return c
}

func (c *RuntimeCacheService) Enabled() bool {
	return c != nil
}

func (c *RuntimeCacheService) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *RuntimeCacheService) GetJSON(ctx context.Context, key string, out any) bool {
	if !c.Enabled() || strings.TrimSpace(key) == "" || out == nil {
		return false
	}
	fullKey := c.key(key)
	if raw, ok := c.getMemory(fullKey); ok {
		return json.Unmarshal(raw, out) == nil
	}
	if c.client != nil {
		raw, err := c.client.Get(ctx, fullKey).Bytes()
		if err == nil {
			if json.Unmarshal(raw, out) != nil {
				return false
			}
			c.setMemory(fullKey, raw, 2*time.Second)
			return true
		}
	}
	return false
}

func (c *RuntimeCacheService) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if !c.Enabled() || strings.TrimSpace(key) == "" || value == nil || ttl <= 0 {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	fullKey := c.key(key)
	c.setMemory(fullKey, raw, ttl)
	if c.client != nil {
		_ = c.client.Set(ctx, fullKey, raw, ttl).Err()
	}
}

// GetObject 返回缓存中的对象。返回值不可变：调用方需要修改时必须先自行拷贝。
func (c *RuntimeCacheService) GetObject(key string) (any, bool) {
	if !c.Enabled() || strings.TrimSpace(key) == "" {
		return nil, false
	}
	fullKey := c.key(key)
	now := time.Now()
	c.mu.RLock()
	item, ok := c.obj[fullKey]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(item.expiresAt) {
		c.mu.Lock()
		delete(c.obj, fullKey)
		c.mu.Unlock()
		return nil, false
	}
	return item.value, true
}

// SetObject 存入一个此后视为不可变的对象。
func (c *RuntimeCacheService) SetObject(key string, value any, ttl time.Duration) {
	if !c.Enabled() || strings.TrimSpace(key) == "" || value == nil || ttl <= 0 {
		return
	}
	fullKey := c.key(key)
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.obj) >= c.limit {
		for k, item := range c.obj {
			if now.After(item.expiresAt) || len(c.obj) >= c.limit {
				delete(c.obj, k)
			}
			if len(c.obj) < c.limit {
				break
			}
		}
	}
	c.obj[fullKey] = runtimeObjectItem{value: value, expiresAt: now.Add(ttl)}
}

func (c *RuntimeCacheService) DeletePrefix(ctx context.Context, prefix string) {
	if !c.Enabled() || strings.TrimSpace(prefix) == "" {
		return
	}
	fullPrefix := c.key(prefix)
	c.deleteMemoryPrefix(fullPrefix)
	if c.client != nil {
		pattern := fullPrefix + "*"
		var cursor uint64
		for {
			keys, next, err := c.client.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				return
			}
			if len(keys) > 0 {
				_ = c.client.Del(ctx, keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				return
			}
		}
	}
}

func (c *RuntimeCacheService) key(key string) string {
	key = strings.TrimLeft(strings.TrimSpace(key), ":")
	if c.prefix == "" {
		return key
	}
	return c.prefix + ":" + key
}

func (c *RuntimeCacheService) getMemory(key string) ([]byte, bool) {
	now := time.Now()
	c.mu.RLock()
	item, ok := c.memory[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(item.expiresAt) {
		c.mu.Lock()
		delete(c.memory, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.raw, true
}

func (c *RuntimeCacheService) setMemory(key string, raw []byte, ttl time.Duration) {
	if ttl <= 0 || len(raw) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.memory) >= c.limit {
		now := time.Now()
		for k, item := range c.memory {
			if now.After(item.expiresAt) || len(c.memory) >= c.limit {
				delete(c.memory, k)
			}
			if len(c.memory) < c.limit {
				break
			}
		}
	}
	c.memory[key] = runtimeCacheItem{raw: append([]byte(nil), raw...), expiresAt: time.Now().Add(ttl)}
}

func (c *RuntimeCacheService) deleteMemoryPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.memory {
		if strings.HasPrefix(key, prefix) {
			delete(c.memory, key)
		}
	}
	for key := range c.obj {
		if strings.HasPrefix(key, prefix) {
			delete(c.obj, key)
		}
	}
}
