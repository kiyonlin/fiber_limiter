package main

import (
	"github.com/gofiber/fiber"
	limiter "github.com/kiyonlin/fiber_limiter"
)

func main() {
	app := fiber.New()

	// 10 requests per second, support 10 burst
	cfg := limiter.Config{
		Limit: 1,
		Burst: 2,
	}

	app.Use(limiter.New(cfg))

	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Welcome!")
	})

	app.Listen(3000)
}
