package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

type RedisCache struct {
	Client *redis.Client
	prefix string
	TTL    time.Duration
}

func NewRedisCache(addr, password, prefix string) *RedisCache {
	return &RedisCache{Client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DialTimeout: 150 * time.Millisecond, ReadTimeout: 150 * time.Millisecond, WriteTimeout: 150 * time.Millisecond, ContextTimeoutEnabled: true, MaxRetries: -1, PoolSize: 16}), prefix: prefix, TTL: 60 * time.Second}
}
func (c *RedisCache) keys(id string) []string {
	return []string{c.prefix + ":vehicle:{" + id + "}:data", c.prefix + ":vehicle:{" + id + "}:generation"}
}

const cacheReadLua = `
local version = tonumber(redis.call('HGET', KEYS[1], 'version') or '0')
local floor = tonumber(redis.call('GET', KEYS[2]) or '0')
if version < floor then redis.call('DEL', KEYS[1]); return false end
return redis.call('HGET', KEYS[1], 'payload')
`
const cachePutLua = `
local incoming = tonumber(ARGV[1])
local floor = tonumber(redis.call('GET', KEYS[2]) or '0')
local cached = tonumber(redis.call('HGET', KEYS[1], 'version') or '0')
if incoming < floor or incoming < cached then return 0 end
if incoming > floor then redis.call('SET', KEYS[2], ARGV[1]) end
redis.call('HSET', KEYS[1], 'version', ARGV[1], 'payload', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1
`
const cacheInvalidateLua = `
local incoming = tonumber(ARGV[1])
local floor = tonumber(redis.call('GET', KEYS[2]) or '0')
if incoming > floor then redis.call('SET', KEYS[2], ARGV[1]); floor = incoming end
local cached = tonumber(redis.call('HGET', KEYS[1], 'version') or '0')
if cached < floor then redis.call('DEL', KEYS[1]) end
return 1
`

func (c *RedisCache) Get(ctx context.Context, id string) (Vehicle, bool, error) {
	value, err := c.Client.Eval(ctx, cacheReadLua, c.keys(id)).Result()
	if err == redis.Nil {
		return Vehicle{}, false, nil
	}
	if err != nil {
		return Vehicle{}, false, fmt.Errorf("cache read failed")
	}
	if value == nil {
		return Vehicle{}, false, nil
	}
	raw, ok := value.(string)
	if !ok {
		return Vehicle{}, false, fmt.Errorf("cache payload invalid")
	}
	var v Vehicle
	if err = json.Unmarshal([]byte(raw), &v); err != nil || v.VehicleID != id {
		return Vehicle{}, false, fmt.Errorf("cache payload invalid")
	}
	return v, true, nil
}
func (c *RedisCache) Put(ctx context.Context, v Vehicle) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Client.Eval(ctx, cachePutLua, c.keys(v.VehicleID), strconv.FormatInt(v.Version, 10), string(raw), c.TTL.Milliseconds()).Err()
}
func (c *RedisCache) Invalidate(ctx context.Context, id string, version int64) error {
	return c.Client.Eval(ctx, cacheInvalidateLua, c.keys(id), strconv.FormatInt(version, 10)).Err()
}
func (c *RedisCache) Close() error { return c.Client.Close() }
