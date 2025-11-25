package ratelimiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

type RateLimiter struct {
	redis  *redis.Client
	config Config
	prefix string
}

type CheckResult struct {
	Allowed         bool
	CurrentRequests int
	CurrentTokens   int
	CurrentDayReqs  int
	ResetMinute     time.Duration
	ResetDay        time.Duration
	RejectionReason string
}

func New(redisClient *redis.Client, config Config) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		config: config,
		prefix: "gemini:ratelimit",
	}
}

func (rl *RateLimiter) CheckAndIncrement(ctx context.Context, tokens int32) (*CheckResult, error) {
	now := time.Now()

	minuteKey := fmt.Sprintf("%s:minute:%s", rl.prefix, now.Format("2006-01-02:15:04"))
	minuteTokenKey := fmt.Sprintf("%s:tokens:minute:%s", rl.prefix, now.Format("2006-01-02:15:04"))
	dayKey := fmt.Sprintf("%s:day:%s", rl.prefix, now.Format("2006-01-02"))

	result := &CheckResult{
		Allowed: true,
	}

	pipe := rl.redis.Pipeline()

	minuteReqsCmd := pipe.Get(ctx, minuteKey)
	minuteTokensCmd := pipe.Get(ctx, minuteTokenKey)
	dayReqsCmd := pipe.Get(ctx, dayKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get current values: %w", err)
	}

	currentMinuteReqs := parseIntOrZero(minuteReqsCmd.Val())
	currentMinuteTokens := parseIntOrZero(minuteTokensCmd.Val())
	currentDayReqs := parseIntOrZero(dayReqsCmd.Val())

	result.CurrentRequests = currentMinuteReqs
	result.CurrentTokens = currentMinuteTokens
	result.CurrentDayReqs = currentDayReqs

	if currentMinuteReqs >= rl.config.RequestsPerMinute {
		result.Allowed = false
		result.RejectionReason = fmt.Sprintf("requests per minute limit exceeded (%d/%d)",
			currentMinuteReqs, rl.config.RequestsPerMinute)
		result.ResetMinute = time.Until(now.Truncate(time.Minute).Add(time.Minute))
		return result, nil
	}

	if currentMinuteTokens+int(tokens) > rl.config.TokensPerMinute {
		result.Allowed = false
		result.RejectionReason = fmt.Sprintf("tokens per minute limit exceeded (%d+%d > %d)",
			currentMinuteTokens, tokens, rl.config.TokensPerMinute)
		result.ResetMinute = time.Until(now.Truncate(time.Minute).Add(time.Minute))
		return result, nil
	}

	if currentDayReqs >= rl.config.RequestsPerDay {
		result.Allowed = false
		result.RejectionReason = fmt.Sprintf("requests per day limit exceeded (%d/%d)",
			currentDayReqs, rl.config.RequestsPerDay)
		result.ResetDay = time.Until(now.Truncate(24 * time.Hour).Add(24 * time.Hour))
		return result, nil
	}

	pipe = rl.redis.Pipeline()

	pipe.Incr(ctx, minuteKey)
	pipe.Expire(ctx, minuteKey, 2*time.Minute)

	pipe.IncrBy(ctx, minuteTokenKey, int64(tokens))
	pipe.Expire(ctx, minuteTokenKey, 2*time.Minute)

	pipe.Incr(ctx, dayKey)
	pipe.Expire(ctx, dayKey, 25*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to increment counters: %w", err)
	}

	result.CurrentRequests++
	result.CurrentTokens += int(tokens)
	result.CurrentDayReqs++
	result.ResetMinute = time.Until(now.Truncate(time.Minute).Add(time.Minute))
	result.ResetDay = time.Until(now.Truncate(24 * time.Hour).Add(24 * time.Hour))

	return result, nil
}

func (rl *RateLimiter) GetCurrentUsage(ctx context.Context) (*CheckResult, error) {
	now := time.Now()

	minuteKey := fmt.Sprintf("%s:minute:%s", rl.prefix, now.Format("2006-01-02:15:04"))
	minuteTokenKey := fmt.Sprintf("%s:tokens:minute:%s", rl.prefix, now.Format("2006-01-02:15:04"))
	dayKey := fmt.Sprintf("%s:day:%s", rl.prefix, now.Format("2006-01-02"))

	pipe := rl.redis.Pipeline()
	minuteReqsCmd := pipe.Get(ctx, minuteKey)
	minuteTokensCmd := pipe.Get(ctx, minuteTokenKey)
	dayReqsCmd := pipe.Get(ctx, dayKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	result := &CheckResult{
		CurrentRequests: parseIntOrZero(minuteReqsCmd.Val()),
		CurrentTokens:   parseIntOrZero(minuteTokensCmd.Val()),
		CurrentDayReqs:  parseIntOrZero(dayReqsCmd.Val()),
		ResetMinute:     time.Until(now.Truncate(time.Minute).Add(time.Minute)),
		ResetDay:        time.Until(now.Truncate(24 * time.Hour).Add(24 * time.Hour)),
	}

	return result, nil
}

func (rl *RateLimiter) Reset(ctx context.Context) error {
	now := time.Now()

	keys := []string{
		fmt.Sprintf("%s:minute:%s", rl.prefix, now.Format("2006-01-02:15:04")),
		fmt.Sprintf("%s:tokens:minute:%s", rl.prefix, now.Format("2006-01-02:15:04")),
		fmt.Sprintf("%s:day:%s", rl.prefix, now.Format("2006-01-02")),
	}

	return rl.redis.Del(ctx, keys...).Err()
}

func parseIntOrZero(s string) int {
	if s == "" {
		return 0
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}
