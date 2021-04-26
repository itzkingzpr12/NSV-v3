package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mediocregopher/radix/v3"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// Cache struct
type Cache struct {
	Client *radix.Pool
}

// GetClient instantiates and returns a connection pool
func GetClient(ctx context.Context, host string, port int, poolSize int) (*radix.Pool, *CacheError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	hostWithPort := fmt.Sprintf("%s:%d", host, port)
	pool, err := radix.NewPool("tcp", hostWithPort, poolSize)

	if err != nil {
		return nil, &CacheError{
			Message: "Unable to create a new cache pool",
			Err:     err,
		}
	}

	return pool, nil
}

// Get a value by key
func (c *Cache) Get(ctx context.Context, key string) (string, *CacheError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var getVal string
	err := c.Client.Do(radix.Cmd(&getVal, "GET", key))

	if err != nil {
		return "", &CacheError{
			Err:     err,
			Message: fmt.Sprintf("Unable to get value from Redis for key: %s", key),
		}
	}

	return getVal, nil
}

// GetStruct gets a struct value by key
func (c *Cache) GetStruct(ctx context.Context, key string, output interface{}) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var getVal string
	getVal, err := c.Get(ctx, key)

	if err != nil {
		return err
	}

	if getVal == "" {
		return nil
	}

	jsonErr := json.Unmarshal([]byte(getVal), output)
	if jsonErr != nil {
		return &CacheError{
			Err:     jsonErr,
			Message: fmt.Sprintf("Unable to unmarshal from Redis for key: %s", key),
		}
	}

	return nil
}

// Set a key:value pair
func (c *Cache) Set(ctx context.Context, key, value string, ttl string) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	err := c.Client.Do(radix.Cmd(nil, "SET", key, value))

	if err != nil {
		return &CacheError{
			Err:     err,
			Message: "Unable to set key:value pair in Redis",
		}
	}

	if ttl != "" {
		exErr := c.Client.Do(radix.Cmd(nil, "EXPIRE", key, ttl))

		if exErr != nil {
			return &CacheError{
				Err:     exErr,
				Message: "Unable to set TTL for key",
			}
		}
	}

	return nil
}

// SetStruct sets a key:value pair
func (c *Cache) SetStruct(ctx context.Context, key string, val interface{}, ttl string) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	jsonVal, jsonErr := json.Marshal(val)
	if jsonErr != nil {
		return &CacheError{
			Err:     jsonErr,
			Message: fmt.Sprintf("Unable to marshal value for key: %s", key),
		}
	}

	err := c.Set(ctx, key, string(jsonVal), ttl)
	if err != nil {
		return err
	}

	return nil
}

// Expire a key
func (c *Cache) Expire(ctx context.Context, key string) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	exErr := c.Client.Do(radix.Cmd(nil, "EXPIRE", key, "-1"))
	if exErr != nil {
		return &CacheError{
			Err:     exErr,
			Message: "Unable to EXPIRE key",
		}
	}

	return nil
}

// TTL gets the TTL of a key
func (c *Cache) TTL(ctx context.Context, key string) (int, *CacheError) {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	var ttl int

	exErr := c.Client.Do(radix.Cmd(&ttl, "TTL", key))
	if exErr != nil {
		return -1, &CacheError{
			Err:     exErr,
			Message: "Unable to get TTL of key",
		}
	}

	return ttl, nil
}
