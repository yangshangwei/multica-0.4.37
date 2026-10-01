package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type PasswordLimit struct {
	Key    string
	Count  int
	Window time.Duration
}
type passwordBucket struct {
	count   int
	expires time.Time
}
type PasswordLimiter struct {
	mu        sync.Mutex
	buckets   map[string]passwordBucket
	lastSweep time.Time
	Redis     *redis.Client
}

func NewPasswordLimiter(rdb *redis.Client) *PasswordLimiter {
	return &PasswordLimiter{buckets: make(map[string]passwordBucket), Redis: rdb}
}

var passwordLimitScript = redis.NewScript(`
local wait = 0
for i,key in ipairs(KEYS) do
 local count = redis.call('INCR', key)
 if count == 1 then redis.call('PEXPIRE', key, ARGV[i*2]) end
 if count > tonumber(ARGV[i*2-1]) then
  local ttl = redis.call('PTTL', key)
  if ttl > wait then wait = ttl end
 end
end
return wait
`)

func (l *PasswordLimiter) Allow(ctx context.Context, limits ...PasswordLimit) (time.Duration, error) {
	if l == nil {
		return 0, errors.New("password limiter unavailable")
	}
	now := time.Now()
	if os.Getenv("MULTICA_PASSWORD_LIMITER_MODE") == "shared" {
		if l.Redis == nil {
			return 0, errors.New("shared password limiter unavailable")
		}
		keys := make([]string, len(limits))
		args := make([]any, 0, len(limits)*2)
		for i, v := range limits {
			keys[i] = fmt.Sprintf("password-limit:%s:%d", HashToken(v.Key), now.UnixNano()/int64(v.Window))
			args = append(args, v.Count, max(int64(1), now.Truncate(v.Window).Add(v.Window).Sub(now).Milliseconds()))
		}
		ms, err := passwordLimitScript.Run(ctx, l.Redis, keys, args...).Int64()
		return time.Duration(ms) * time.Millisecond, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = make(map[string]passwordBucket)
	}
	if now.Sub(l.lastSweep) >= time.Minute {
		for k, b := range l.buckets {
			if !now.Before(b.expires) {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}
	newKeys := 0
	for _, v := range limits {
		if _, ok := l.buckets[v.Key]; !ok {
			newKeys++
		}
	}
	if len(l.buckets)+newKeys > 50000 {
		return 0, errors.New("password limiter capacity reached")
	}
	var wait time.Duration
	for _, v := range limits {
		b := l.buckets[v.Key]
		if !now.Before(b.expires) {
			b = passwordBucket{expires: now.Truncate(v.Window).Add(v.Window)}
		}
		b.count++
		l.buckets[v.Key] = b
		if b.count > v.Count && b.expires.Sub(now) > wait {
			wait = b.expires.Sub(now)
		}
	}
	return wait, nil
}

// PasswordClientIP trusts forwarded headers only across an explicitly trusted chain.
func PasswordClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	trusted := func(s string) bool {
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip == nil {
			return false
		}
		for _, cidr := range strings.Split(os.Getenv("MULTICA_TRUSTED_PROXIES"), ",") {
			_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
			if err == nil && network.Contains(ip) {
				return true
			}
		}
		return false
	}
	if trusted(host) {
		chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for i := len(chain) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(chain[i])
			if net.ParseIP(candidate) == nil {
				break
			}
			host = candidate
			if !trusted(host) {
				break
			}
		}
	}
	return host
}
