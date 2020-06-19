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
