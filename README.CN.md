# 前言
后端的`api`接口一般都需要限制访问频率，其实现算法有[令牌桶](https://en.wikipedia.org/wiki/Token_bucket)，[漏桶](https://en.wikipedia.org/wiki/Leaky_bucket)等等。其中令牌桶支持突发流量，更合适访问流量整形。

令牌桶算法不再赘述，目前有两个`golang`限流器库，[github.com/juju/ratelimit](https://github.com/juju/ratelimit)和[golang.org/x/time/rate](https://github.com/golang/time)，采用了延迟计算的方式实现令牌桶算法。本文主要是基于[golang.org/x/time/rate](https://github.com/golang/time)的限流器进行实现，有两篇相关联的文章《[Golang限流器time/rate使用介绍](https://zhuanlan.zhihu.com/p/89820414)》和《[Golang限流器time/rate实现剖析](https://zhuanlan.zhihu.com/p/90206074)》，有兴趣的同学可以查看一下。

# 限流响应头
限流响应头主要涉及四个字段：

- 未超出频率限制时使用：
    - `X-RateLimit-Limit`：请求限制总量，对应了令牌桶算法中的突发值`Burst`
    - `X-RateLimit-Remaining`：目前还可以请求的次数
    - `X-RateLimit-Reset`：多少秒才能恢复到满桶的状态
- 超出频率限制时使用：
    - `Retry-After`：多少秒之后可以重试

[golang.org/x/time/rate](https://github.com/golang/time)的[Reserve](https://github.com/golang/time/blob/master/rate/rate.go#L192-L193)方法返回一个`*Reservation`对象，根据它的[Delay](https://github.com/golang/time/blob/master/rate/rate.go#L121-L22)方法，我们可以知道本次请求是否超出了频率限制：
- `delay`大于0，表示需要等待，即超出了频率限制。此时，我们根据`delay`转换成秒数(至少一秒)即可。
- `delay`等于0，表示不需要等待，即未超出频率限制。此时，我们除了`Burst`，无法获取更多的有效信息。

# 为`Reservation`增加信息
因为`Reservation`缺少可以转化为`X-RateLimit-Remaining`和`X-RateLimit-Reset`的信息，我们需要在调用`Reserve`时，保存一些相关信息([源码位置]())：

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

我们先看`r.remainedTokens = int(math.Floor(tokens + 1e-9))`，它保存了调用`Reserve`时，`limiter`剩余的`token`值。因为`tokens`是`float64`类型，会丢失一点精度，我们需要补全精度后，转为`int`类型。

再看`r.reset = r.limit.durationFromTokens(float64(r.lim.burst) - tokens)`，`float64(r.lim.burst) - tokens`是已经消耗掉的`token`数量，我们根据这个数量，转换为时长，即恢复这么多`token`还需要多长时间。

# 实现`fiber`频率限制中间件
根据[fiber](https://gofiber.io)中间件的实现规则，我们先创建一个配置结构：

```go
type Config struct {
	// Filter 定义了是否跳过中间件的方法，默认是nil
	Filter func(*fiber.Ctx) bool
	// Limit 定义了请求频率的最大值，表示每秒Limit个请求，默认值是 10
	Limit int
	// Burst 是最大突发值，默认值是 10
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
	//   c.Status(cfg.StatusCode).Format(cfg.Message)
	// }
	Handler func(*fiber.Ctx)
}
```
