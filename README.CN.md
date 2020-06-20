# 前言
后端的`api`接口一般都需要限制访问频率，一般的实现算法有[令牌桶](https://en.wikipedia.org/wiki/Token_bucket)，[漏桶](https://en.wikipedia.org/wiki/Leaky_bucket)等等。其中令牌桶支持突发流量，更合适访问流量整形。

关于令牌桶算法这里不再赘述，目前有两个`golang`限流器库，[github.com/juju/ratelimit](https://github.com/juju/ratelimit)和[golang.org/x/time/rate](https://github.com/golang/time)，采用了延迟计算的方式实现令牌桶算法。本文主要是基于[golang.org/x/time/rate](https://github.com/golang/time)的限流器进行实现，有两篇相关的文章《[Golang限流器time/rate使用介绍](https://zhuanlan.zhihu.com/p/89820414)》和《[Golang限流器time/rate实现剖析](https://zhuanlan.zhihu.com/p/90206074)》，有兴趣的同学可以查看一下。

# 限流响应头
限流响应头主要涉及四个字段：

- 未超出频率限制时：
    - `X-RateLimit-Limit`：请求限制总量，对应了令牌桶算法中的突发值`Burst`
    - `X-RateLimit-Remaining`：目前还可以请求的次数
    - `X-RateLimit-Reset`：多少秒才能恢复到满桶的状态
- 超出频率限制时：
    - `Retry-After`：多少秒之后可以重试

[golang.org/x/time/rate](https://github.com/golang/time)的[Reserve](https://github.com/golang/time/blob/master/rate/rate.go#L192-L193)方法返回一个`*Reservation`对象，根据它的[Delay](https://github.com/golang/time/blob/master/rate/rate.go#L121-L122)方法，我们可以知道本次请求是否超出了频率限制：
- `delay`大于0，表示需要等待，即超出了频率限制。此时，我们根据`delay`转换成秒数(至少一秒)即可。
- `delay`等于0，表示不需要等待，即未超出频率限制。此时，我们除了`Burst`，无法获取更多的有效信息。

# 为`Reservation`增加信息
因为`Reservation`缺少可以转化为`X-RateLimit-Remaining`和`X-RateLimit-Reset`的信息，我们需要在调用`Reserve`时，保存一些相关数据([源码位置](https://github.com/kiyonlin/rate/blob/master/rate.go#L366-L369))：

```go
if ok {
    r.tokens = n
    r.timeToAct = now.Add(waitDuration)
    // store remaining tokens as integer
    // 1e-9 used to solve the problem of missing precision
    r.remainedTokens = int(math.Floor(tokens + 1e-9))
    r.reset = r.limit.durationFromTokens(float64(r.lim.burst) - tokens)
}
```

我们先看`r.remainedTokens = int(math.Floor(tokens + 1e-9))`，它保存了调用`Reserve`时，`limiter`剩余的整数`token`值。因为`tokens`是`float64`类型，会丢失一点精度，我们需要补全精度后，转为`int`类型。

再看`r.reset = r.limit.durationFromTokens(float64(r.lim.burst) - tokens)`，`float64(r.lim.burst) - tokens`是已经消耗掉的`token`数量，我们根据这个数量，转换为时长，即恢复这么多`token`还需要多长时间。

# 实现`fiber`频率限制中间件
## Config
根据[fiber](https://gofiber.io)中间件的实现规则，我们先创建一个配置结构：

```go
type Config struct {
	// Filter 定义了是否跳过中间件的方法，默认是nil
	Filter func(*fiber.Ctx) bool
	// Limit 定义了请求频率的最大值，表示每秒Limit个请求，默认值是 10
	Limit int
	// Burst 是最大突发值，默认值是 10
	Burst int
	// Message 响应消息，默认值是 "Too many requests, please try again later."
	Message string
	// StatusCode 状态码，默认值是 429
	StatusCode int
	// Key 允许用户使用自定义handler生成自定义的 key，默认值是 
	// func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string
	// Handler 触发频率限制时调用的 handler， 默认值是
	// func(c *fiber.Ctx) {
	//   c.Status(cfg.StatusCode).Format(cfg.Message)
	// }
	Handler func(*fiber.Ctx)
}
```

利用这些配置，用户可以根据自己的需求使用中间件。

## New
`func New(config ...Config) func(*fiber.Ctx)`是中间件的工厂函数，根据用户传入的配置，返回一个`fiber`中间件。

我们需要缓存下所有的限制器对象，存放在`limiters`变量中，并用`mu`控制并发访问：

```go
var (
	limiters = make(map[string]*rate.Limiter)
	mu       sync.Mutex
)
```

当`api`接口收到请求时，获取限制器的`key`，再根据`key`获取限制器，没找到的话根据配置为`key`新建一个限制器：

```go
key := cfg.Key(c)

mu.Lock()
lim, ok := limiters[key]
if !ok {
    // Get a default limiter
    lim = rate.NewLimiter(rate.Limit(cfg.Limit), cfg.Burst)
    limiters[key] = lim
}
mu.Unlock()
```

然后，我们调用`lim.Reserve`获取一个`*Reservation`对象，根据其`Delay()`返回值确定有没有超出频率限制：

```go
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
```

其中`X-RateLimit-Reset`和`Retry-After`的值都转成秒为单位的整数，这是一个相对当前调用时间的值，客户端可以根据这个值作出相应的操作。

## Set
最后，我们提供`func Set(key string, lim *rate.Limiter)`方法，用户可以根据自己的需求提前设置不同的限制器。比如针对不同的`api`用户，设置不同的访问频率以及突发值。

# 总结
本文主要针对`api`接口的频率限制需求，在[golang.org/x/time/rate](https://github.com/golang/time)的基础上为`Reservation`对象增加方法，完善响应头中的信息，并应用到[fiber](https://gofiber.io)框架中。

最后，我们可以发现基于令牌桶延迟计算实现频率限制的好处：

- 不需要定时器
- 不需要后台`goroutine`
- 不需要队列
- 支持突发流量

仓库资源：

- 基于[golang.org/x/time/rate](https://github.com/golang/time)的[rate](https://github.com/kiyonlin/rate)包
- [fiber limiter](https://github.com/kiyonlin/fiber_limiter)

以上。
