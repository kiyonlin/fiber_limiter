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
		c.Send("Welcome VIP!")
	})

	app.Listen(3000)
}
