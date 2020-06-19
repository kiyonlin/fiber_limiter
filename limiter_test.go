package fiber_limiter

import (
	"github.com/gofiber/fiber"
	"github.com/kiyonlin/rate"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func Test_Concurrency(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{Burst: 100}))
	app.Get("/", func(ctx *fiber.Ctx) {
		// random delay between the requests
		time.Sleep(time.Duration(rand.Intn(10000)) * time.Microsecond)
		ctx.Send("Hello tester!")
	})

	headers := []string{"X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Reset"}

	var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup) {
		defer wg.Done()
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Unexpected status code %v", resp.StatusCode)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil || "Hello tester!" != string(body) {
			t.Fatalf("Unexpected body %v", string(body))
		}

		for _, key := range headers {
			if len(resp.Header[key]) != 1 {
				t.Fatalf("header %s not found", key)
			}
		}
	}

	for i := 0; i <= 50; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()
}

func Test_Retry_After(t *testing.T) {
	key := "limited"

	lim := rate.NewLimiter(0.1, 1)
	// Reserve the default token
	lim.Reserve()
	Set(key, lim)

	app := fiber.New()
	app.Use(New(Config{Key: func(_ *fiber.Ctx) string { return key }}))
	app.Get("/", func(c *fiber.Ctx) {})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Unexpected status code %v", resp.StatusCode)
	}

	if h := resp.Header[fiber.HeaderRetryAfter]; len(h) == 0 {
		t.Fatalf("header %s not found", fiber.HeaderRetryAfter)
	} else {
		if h[0] != "10" {
			t.Fatalf("Unexpected retry after %v", h[0])
		}
	}
}

func Test_Skip_Middleware(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{Filter: func(_ *fiber.Ctx) bool {
		return true
	}}))
	app.Get("/", func(c *fiber.Ctx) {})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	headers := []string{"X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Reset", fiber.HeaderRetryAfter}

	for _, key := range headers {
		if len(resp.Header[key]) != 0 {
			t.Fatalf("found header %s ", key)
		}
	}
}
