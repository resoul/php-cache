package ratelimiter

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Redis should be available for tests")

	err = client.FlushDB(ctx).Err()
	require.NoError(t, err)

	return client
}

func TestRateLimiter_RequestsPerMinute(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, Config{
		RequestsPerMinute: 2,
		TokensPerMinute:   1000,
		RequestsPerDay:    100,
	})

	ctx := context.Background()

	result, err := limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, result.CurrentRequests)

	result, err = limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 2, result.CurrentRequests)

	result, err = limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.RejectionReason, "requests per minute")
}

func TestRateLimiter_TokensPerMinute(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, Config{
		RequestsPerMinute: 100,
		TokensPerMinute:   50,
		RequestsPerDay:    1000,
	})

	ctx := context.Background()

	result, err := limiter.CheckAndIncrement(ctx, 30)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 30, result.CurrentTokens)

	result, err = limiter.CheckAndIncrement(ctx, 25)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.RejectionReason, "tokens per minute")
}

func TestRateLimiter_RequestsPerDay(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, Config{
		RequestsPerMinute: 100,
		TokensPerMinute:   10000,
		RequestsPerDay:    2,
	})

	ctx := context.Background()

	result, err := limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	result, err = limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	result, err = limiter.CheckAndIncrement(ctx, 10)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.RejectionReason, "requests per day")
}

func TestRateLimiter_GetCurrentUsage(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    50,
	})

	ctx := context.Background()

	_, err := limiter.CheckAndIncrement(ctx, 20)
	require.NoError(t, err)
	_, err = limiter.CheckAndIncrement(ctx, 30)
	require.NoError(t, err)

	usage, err := limiter.GetCurrentUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, usage.CurrentRequests)
	assert.Equal(t, 50, usage.CurrentTokens)
	assert.Equal(t, 2, usage.CurrentDayReqs)
}

func TestRateLimiter_Reset(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    50,
	})

	ctx := context.Background()

	_, err := limiter.CheckAndIncrement(ctx, 20)
	require.NoError(t, err)

	err = limiter.Reset(ctx)
	require.NoError(t, err)

	usage, err := limiter.GetCurrentUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, usage.CurrentRequests)
	assert.Equal(t, 0, usage.CurrentTokens)
	assert.Equal(t, 0, usage.CurrentDayReqs)
}
