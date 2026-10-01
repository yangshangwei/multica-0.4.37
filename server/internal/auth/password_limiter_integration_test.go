package auth

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// The opt-in URL points at a disposable Redis; no keys outside this test's
// unique fixed-window bucket are modified.
func TestPasswordLimiterSharedAtomicAdmission(t *testing.T) {
	url := os.Getenv("MULTICA_TEST_REDIS_URL")
	if url == "" {
		t.Skip("MULTICA_TEST_REDIS_URL not configured")
	}
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "shared")
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("test-account:%d", time.Now().UnixNano())
	window := 24 * time.Hour
	start := time.Now().UnixNano() / int64(window)
	t.Cleanup(func() {
		for slot := start; slot <= start+1; slot++ {
			_ = rdb.Del(context.Background(), fmt.Sprintf("password-limit:%s:%d", HashToken(key), slot)).Err()
		}
	})
	instances := []*PasswordLimiter{NewPasswordLimiter(rdb), NewPasswordLimiter(rdb)}
	var allowed atomic.Int32
	var failed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 48; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wait, err := instances[i%2].Allow(ctx, PasswordLimit{Key: key, Count: 10, Window: window})
			if err != nil {
				failed.Add(1)
				return
			}
			if wait == 0 {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if failed.Load() != 0 || allowed.Load() != 10 {
		t.Fatalf("two instances admitted %d/48, errors=%d; want exactly10", allowed.Load(), failed.Load())
	}
	broken := redis.NewClient(opts)
	_ = broken.Close()
	if _, err := NewPasswordLimiter(broken).Allow(ctx, PasswordLimit{Key: key, Count: 10, Window: window}); err == nil {
		t.Fatal("Redis failure silently allowed a password attempt")
	}
}

func TestPasswordLimiterDoesNotEvictActiveBucketsAtCapacity(t *testing.T) {
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	limiter := NewPasswordLimiter(nil)
	for i := 0; i < 50000; i++ {
		limiter.buckets[fmt.Sprint(i)] = passwordBucket{count: 10, expires: time.Now().Add(time.Hour)}
	}
	if _, err := limiter.Allow(context.Background(), PasswordLimit{Key: "new", Count: 10, Window: time.Minute}); err == nil {
		t.Fatal("full limiter admitted a new key")
	}
	if len(limiter.buckets) != 50000 || limiter.buckets["0"].count != 10 {
		t.Fatal("capacity pressure evicted an active limit")
	}
}
