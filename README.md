# 限流工具【令牌桶实现】


用法：

- 初始化：

```go
func NewBucket(capacity, quantum int64, fillInterval time.Duration, options ...OptionF) (bucket *Bucket) 
```

NewBucket 返回一个新的令牌桶，每 fillInterval 个时间间隔，填充 quantum 个令牌的，直到给定的最大容量。桶初始化后就是满的

`options` 可选 `WithClock(mockClock)` 可以用其他时钟代替，方便 mock 测试 ，`WithProhibitOverflow()` 禁止桶溢出，此时退换成漏桶算法

- 获取一定数量的令牌

```go
func (b *Bucket) Take(count int64) 
```

Take 获取一定数量的令牌，如果当前桶中令牌数量不够，则阻塞直到获取成功

如果不使用 prohibitOverflow 选项， err 永远返回 nil ，如果使用 prohibitOverflow 选项，则获取超过容量的令牌数则会返回 ErrProhibitOverflow

- 在规定的等待时间内获取一定数量令牌


```go
func (b *Bucket) TryTake(count int64, maxWait time.Duration) (succ bool)
```

从桶中获取一定数量的令牌，如果桶中令牌数充足则立即返回 true,或者桶中令牌数不够但是预计等待时间 <= maxWait 则阻塞到实际等待时间后并返回 true ,否则返回 false

如果使用 prohibitOverflow 选项并且count超过了capacity 则永远返回false

- 尝试获取一定数量的令牌，不阻塞

```go
func (b *Bucket) TakeAvailable(count int64) (realCount int64) 
```

尝试获取 count 个令牌，如果桶中令牌不够也不会阻塞，返回值告诉调用方实际上获取了多少个令牌

realCount 不会大于容量

- 当前可用的令牌数量
```go
func (b *Bucket) Available() (available int64)
```
返回当前可用的令牌数量


