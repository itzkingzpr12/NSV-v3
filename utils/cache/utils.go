package cache

import (
	"context"
	"fmt"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/configs"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// GenerateKey func
func GenerateKey(base string, id interface{}) string {
	return fmt.Sprintf("%s:%s", base, fmt.Sprint(id))
}

// GetCache func
func GetCache(ctx context.Context, ca *Cache, settings configs.CacheSetting, cacheKey string, output interface{}) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if !settings.Enabled {
		return nil
	}

	cacheErr := ca.GetStruct(ctx, cacheKey, &output)
	return cacheErr
}

// SetCache func
func SetCache(ctx context.Context, ca *Cache, settings configs.CacheSetting, cacheKey string, input interface{}) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if !settings.Enabled {
		return nil
	}

	cacheErr := ca.SetStruct(ctx, cacheKey, &input, settings.TTL)
	return cacheErr
}

// ExpireCache func
func ExpireCache(ctx context.Context, ca *Cache, settings configs.CacheSetting, cacheKey string) *CacheError {
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	if !settings.Enabled {
		return nil
	}

	cacheErr := ca.Expire(ctx, cacheKey)
	return cacheErr
}
