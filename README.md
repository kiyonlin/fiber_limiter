# Limiter
fiber_limiter is based on [rate](https://github.com/kiyonlin/rate) which forks of [golang.org/x/time/rate](https://github.com/golang/time). The core algorithm is delay calculate of [token bucket](https://en.wikipedia.org/wiki/Token_bucket) which supports burst.

## Install
```
go get -u github.com/gofiber/fiber
go get -u github.com/kiyonlin/fiber-limiter
```

## Config
| Property | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| Filter | `func(*fiber.Ctx) bool` | Defines a function to skip middleware. | `nil` |
| Limit | `int` | Defines the maximum frequency of requests and is represented as integer of requests per second. | `10` |
| Burst | `int` | Maximum burst size. | `10` |
| Message | `string` | Response Message. | `"Too many requests, please try again later."` |
| StatusCode | `int` | Status Code. | `429 Too Many Requests` |
| Key | `func(*fiber.Ctx) string` | Allows to use a custom handler to create custom keys. | `func(c *fiber.Ctx) string { return c.IP() }` |
| Handler | `func(*fiber.Ctx)` | Is called when a request hits the limit. | `func(c *fiber.Ctx) { c.Status(cfg.StatusCode).Format(cfg.Message) }` |

## Example
```go
package main

import (
	"github.com/gofiber/fiber"
	limiter "github.com/kiyonlin/fiber_limiter"
)

func main() {
	app := fiber.New()

	// 10 requests per second, support 10 burst
	cfg := limiter.Config{
		Limit: 10,
		Burst: 10,
	}

	app.Use(limiter.New(cfg))

	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Welcome!")
	})

	app.Listen(3000)
}

```
## Test
```curl
curl http://localhost:3000 -I
...
< HTTP/1.1 200 OK
< Date: Fri, 19 Jun 2020 05:43:51 GMT
< Content-Type: text/plain; charset=utf-8
< Content-Length: 8
< X-Ratelimit-Limit: 2
< X-Ratelimit-Remaining: 1
< X-Ratelimit-Reset: 1
...

curl http://localhost:3000
curl http://localhost:3000
curl http://localhost:3000
curl http://localhost:3000

curl http://localhost:3000 -I
...
< HTTP/1.1 429 Too Many Requests
< Date: Fri, 19 Jun 2020 05:43:52 GMT
< Content-Type: text/html
< Content-Length: 49
< Retry-After: 3
...
```

## Custom limiter
We can set custom limiter for specific key so that every user can have a *different* limiter:

```go
package main

import (
	"github.com/gofiber/fiber"
	limiter "github.com/kiyonlin/fiber_limiter"
	"github.com/kiyonlin/rate"
)

func main() {
	app := fiber.New()

	// 10 requests per second, support 10 burst
	limiter.Set("127.0.0.1", rate.NewLimiter(10, 100))

	app.Use(limiter.New())

	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Welcome, VIP!")
	})

	app.Listen(3000)
}

```
