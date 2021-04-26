package logging

import (
	"context"

	"go.uber.org/zap"
)

// Key string
type Key string
type dummy struct{}

const trackedKeys = Key("tracked-keys")

// AddValues creates a new immutable context which includes the new key and value
func AddValues(ctx context.Context, values ...zap.Field) context.Context {
	keys := copyMap(getKeys(ctx))
	for _, val := range values {
		key := Key(val.Key)
		keys[key] = dummy{}

		ctx = context.WithValue(ctx, trackedKeys, keys)
		ctx = context.WithValue(ctx, key, val)
	}

	return ctx
}

// GetValues returns a map of all values stored in the current context
func GetValues(ctx context.Context) map[string]zap.Field {
	values := make(map[string]zap.Field)
	keys := getKeys(ctx)

	for k := range keys {
		if v, ok := ctx.Value(k).(zap.Field); ok {
			values[string(k)] = v
		}
	}

	return values
}

// copyMap makes a defensive copy to protect against concurrent writes
func copyMap(from map[Key]dummy) map[Key]dummy {
	v := dummy{}
	to := make(map[Key]dummy)

	for k := range from {
		to[k] = v
	}

	return to
}

// getKeys parses all the values from the context and returns them as a map
func getKeys(ctx context.Context) map[Key]dummy {
	keys := make(map[Key]dummy)

	if k, ok := ctx.Value(trackedKeys).(map[Key]dummy); ok {
		keys = k
	}

	return keys
}

// GetValuesSlice returns a slice of all values stored in the current context
func GetValuesSlice(ctx context.Context) []zap.Field {
	values := []zap.Field{}
	keys := getKeys(ctx)

	for k := range keys {
		if v, ok := ctx.Value(k).(zap.Field); ok {
			values = append(values, v)
		}
	}

	return values
}
