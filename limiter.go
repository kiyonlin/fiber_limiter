package fiber_limiter

import (
	"github.com/gofiber/fiber"
	"github.com/kiyonlin/rate"
	"math"
	"strconv"
	"sync"
)

var (
	limiters = make(map[string]*rate.Limiter)
	mu       sync.Mutex
)

// Config ...
type Config struct {
	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Filter func(*fiber.Ctx) bool
	// Limit defines the maximum frequency of requests.
	// Limit is represented as integer of requests per second.
	// Default: 10
	Limit int
	// Burst is maximum burst size
	// Default: 10
	Burst int
	// Message
	// default: "Too many requests, please try again later."
	Message string
	// StatusCode
	// Default: 429 Too Many Requests
	StatusCode int
	// Key allows to use a custom handler to create custom keys
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string
	// Handler is called when a request hits the limit
	// Default: func(c *fiber.Ctx) {
	//   c.Status(cfg.StatusCode).SendString(cfg.Message)
	// }
	Handler func(*fiber.Ctx)
}

// New ...
func New(config ...Config) func(*fiber.Ctx) {
	// Init config
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Limit == 0 {
		cfg.Limit = 10
	}

	if cfg.Burst == 0 {
		cfg.Burst = 10
	}

	if cfg.Message == "" {
		cfg.Message = "Too many requests, please try again later."
	}
	if cfg.StatusCode == 0 {
		cfg.StatusCode = 429
	}
	if cfg.Key == nil {
		cfg.Key = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}
	if cfg.Handler == nil {
		cfg.Handler = func(c *fiber.Ctx) {
			c.Status(cfg.StatusCode).Format(cfg.Message)
		}
	}

	return func(c *fiber.Ctx) {
		// Filter request to skip middleware
		if cfg.Filter != nil && cfg.Filter(c) {
			c.Next()
			return
		}

		key := cfg.Key(c)

		mu.Lock()
		lim, ok := limiters[key]
		if !ok {
			// Get a default limiter
			lim = rate.NewLimiter(rate.Limit(cfg.Limit), cfg.Burst)
			limiters[key] = lim
		}
		mu.Unlock()

		// Try to request
		r := lim.Reserve()

		// Check reservation's delay
		if d := r.Delay(); d > 0 {
			cfg.Handler(c)

			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc7231#section-7.1.3
			// Set second value(at least one) to Retry-After header
			c.Set(fiber.HeaderRetryAfter, strconv.FormatInt(int64(math.Ceil(d.Seconds())), 10))

			return
		}

		// We can continue, update RateLimit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(lim.Burst()))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(r.RemainedTokens()))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(int64(math.Ceil(r.Reset().Seconds())), 10))
		// Bye!
		c.Next()
	}
}

// Set sets custom limiter for a specific key
func Set(key string, lim *rate.Limiter) {
	mu.Lock()
	limiters[key] = lim
	mu.Unlock()
}
