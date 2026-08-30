package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
)

// incrScript atomically increments a counter and applies its expiration on
// the first increment. Plain INCR followed by EXPIRE would leave a permanent
// key behind when the process dies between the two commands.
const incrScript = `local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return n`

// Valkey is the Cache implementation backed by a Valkey /
// Redis-compatible server, so per-process state (rate limiting, SSO state,
// content caching) is shared across application instances. Use NewValkey to
// connect.
type Valkey struct {
	client valkey.Client
}

// Get returns the value stored under key, or ok=false for a missing key.
func (c *Valkey) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Do(ctx, c.client.B().Get().Key(keyPrefix+key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("cache get %q: %w", key, err)
	}
	return value, true, nil
}

// Set stores value under key. A non-positive ttl means no expiration.
func (c *Valkey) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	set := c.client.B().Set().Key(keyPrefix + key).Value(value)
	var completed valkey.Completed
	if ttl > 0 {
		completed = set.PxMilliseconds(ttl.Milliseconds()).Build()
	} else {
		completed = set.Build()
	}
	if err := c.client.Do(ctx, completed).Error(); err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}
	return nil
}

// Delete removes key if present.
func (c *Valkey) Delete(ctx context.Context, key string) error {
	if err := c.client.Do(ctx, c.client.B().Del().Key(keyPrefix+key).Build()).Error(); err != nil {
		return fmt.Errorf("cache delete %q: %w", key, err)
	}
	return nil
}

// GetDel atomically returns and removes the value stored under key.
func (c *Valkey) GetDel(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Do(ctx, c.client.B().Getdel().Key(keyPrefix+key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("cache getdel %q: %w", key, err)
	}
	return value, true, nil
}

// Incr atomically increments the counter stored at key, applying ttl only
// when the call creates the counter (see incrScript).
func (c *Valkey) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := c.client.Do(ctx, c.client.B().Eval().
		Script(incrScript).
		Numkeys(1).
		Key(keyPrefix+key).
		Arg(strconv.FormatInt(ttl.Milliseconds(), 10)).
		Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("cache incr %q: %w", key, err)
	}
	return n, nil
}

// Close closes the connection pool.
func (c *Valkey) Close() {
	c.client.Close()
}
