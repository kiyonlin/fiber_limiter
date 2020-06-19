module github.com/kiyonlin/fiber_limiter

go 1.14

require (
	github.com/gofiber/fiber v1.11.1
	github.com/kiyonlin/rate v0.2.1
)

replace (
	github.com/kiyonlin/rate => ../time-rate
)
