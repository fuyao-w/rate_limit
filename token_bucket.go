package rate_limit

/*
	Copyright [2022] [wangfuyao]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
import (
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrOverflow = errors.New("prohibit overflow")
)

// TokenBucket 令牌桶算法
type TokenBucket struct {
	capacity        int64         // 桶容量
	fillInterval    time.Duration // 填充间隔
	quantum         int64         // 单次填充令牌数
	availableTokens int64         // 当前可用的令牌数
	createTime      time.Time     // 桶被创建的时间，用于计算间隔次数
	lastTick        int64         // 最新的间隔次数，当需要等待的时候计算截止时间
	*Options
	mu *sync.Mutex
}

type Options struct {
	clock            Clock // 时钟 用户可以传入自定义的时钟用于mock测试
	prohibitOverflow bool
}

func NewBucket(capacity, quantum int64, fillInterval time.Duration, options ...OptionF) (bucket *TokenBucket) {
	if capacity <= 0 {
		panic("capacity is not > 0")
	}
	if quantum <= 0 {
		panic("quantum is not > 0")
	}
	if fillInterval <= 0 {
		panic("fill interval is not > 0")
	}
	opt := loadOptions(options...)
	bucket = &TokenBucket{
		Options:         opt,
		capacity:        capacity,
		fillInterval:    fillInterval,
		quantum:         quantum,
		createTime:      opt.clock.Now(),
		mu:              new(sync.Mutex),
		availableTokens: capacity,
	}

	return
}

const infinityDuration time.Duration = math.MaxInt64 // 一直阻塞直到获取令牌

type OptionF func(opt *Options)

func loadOptions(options ...OptionF) (opt *Options) {
	opt = new(Options)
	for _, f := range options {
		f(opt)
	}
	if opt.clock == nil {
		opt.clock = new(standardClock)
	}

	return
}

func WithClock(clock Clock) OptionF {
	return func(opt *Options) {
		opt.clock = clock
	}
}

// WithProhibitOverflow 禁止溢出，此时不允许获取超过容量的令牌，退化成漏桶算法
func WithProhibitOverflow() OptionF {
	return func(opt *Options) {
		opt.prohibitOverflow = true
	}
}

// Available 当前可用的令牌数量
func (b *TokenBucket) Available() (available int64) {
	b.doWithLock(func() time.Duration {
		b.adjustAvailableTokens(b.currentTick(b.now()))
		available = b.availableTokens
		return 0
	})
	return
}

// doWithLock  封装上锁的函数处理
func (b *TokenBucket) doWithLock(f func() time.Duration) {
	var waitTime time.Duration
	func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		waitTime = f()
	}()
	b.clock.Sleep(waitTime)
}

// Take 获取一定数量的令牌，如果当前桶中令牌数量不够，则阻塞直到获取成功
// 如果不使用 prohibitOverflow 选项， err 永远返回 nil
// 如果使用 prohibitOverflow 选项，则获取超过容量的令牌数则会返回 ErrProhibitOverflow
func (b *TokenBucket) Take(count int64) (err error) {
	b.doWithLock(func() time.Duration {
		if waitTime, succ := b.take(count, b.now(), infinityDuration); succ {
			return waitTime
		}
		err = ErrOverflow
		return 0
	})
	return
}

// TakeAvailable 尝试获取 count 个令牌，如果桶中令牌不够也不会阻塞，返回值告诉调用方实际上获取了多少个令牌
// realCount 不会大于容量
func (b *TokenBucket) TakeAvailable(count int64) (realCount int64) {
	b.doWithLock(func() time.Duration {
		realCount = b.takeAvailable(b.now(), count)
		return 0
	})
	return
}

// takeAvailable 尝试获取 count 个令牌，如果桶中令牌不够只获取当前可用的令牌数量
func (b *TokenBucket) takeAvailable(now time.Time, count int64) (realCount int64) {
	if count <= 0 {
		return
	}
	b.adjustAvailableTokens(b.currentTick(now))

	if b.availableTokens < count {
		realCount = b.availableTokens
		b.availableTokens = 0
		return
	}
	b.availableTokens -= count
	return count
}

// TryTake 从桶中获取一定数量的令牌，如果桶中令牌数充足则立即返回 true,或者桶中令牌数不够但是预计等待时间 <= maxWait 则阻塞到实际等待时间后并返回 true ,否则返回 false
// 如果使用 prohibitOverflow 选项并且count超过了capacity 则永远返回false
func (b *TokenBucket) TryTake(count int64, maxWait time.Duration) (succ bool) {
	b.doWithLock(func() (waitTime time.Duration) {
		waitTime, succ = b.take(count, b.now(), maxWait)
		if succ {
			return
		}
		return 0
	})
	return
}

func (b *TokenBucket) now() time.Time {
	return b.clock.Now()
}

// currentTick 返回当前已经经过了多少填充间隔，用于计算桶里应该填充的令牌数量
func (b *TokenBucket) currentTick(now time.Time) int64 {
	return int64(now.Sub(b.createTime) / b.fillInterval)
}

// adjustAvailableTokens 调整当前桶中应该有的令牌数量
func (b *TokenBucket) adjustAvailableTokens(tick int64) {
	lastTick := b.lastTick
	b.lastTick = tick
	if b.availableTokens >= b.capacity {
		return
	}
	b.availableTokens += (tick - lastTick) * b.quantum
	if b.availableTokens > b.capacity {
		b.availableTokens = b.capacity
	}

}

// take 获取一定数量的令牌，可以选择在桶中令牌数不够情况下的最长等待时间，如果满足条件则返回实际需要等待的时间，和是否获取成功
// count 想获取的令牌数量
// now 当前时间
// maxWait 如果桶内令牌数不够可以等待的最长时间
// waiTime 需要等待的时间
// succ 是否获取成功，桶中数量足够或者 waiTime <= maxWait 才会返回 true
// 注意，如果 succ 返回 true 则代表已经从桶中取出了一些令牌，不会再放回桶中
func (b *TokenBucket) take(count int64, now time.Time, maxWait time.Duration) (waiTime time.Duration, succ bool) {
	if count <= 0 {
		return 0, true
	}
	if b.prohibitOverflow && count > b.capacity {
		return 0, false
	}
	// 先根据当前 tick 数量计算出最新的可用令牌数
	tick := b.currentTick(now)
	b.adjustAvailableTokens(tick)
	newAvailable := b.availableTokens - count
	if newAvailable >= 0 { //如果当前令牌数足够，则可以直接返回
		b.availableTokens = newAvailable
		return 0, true
	}
	//令牌数量不够的时候就需要计算需要等待的时间，并返回
	endTick := tick + (-newAvailable+b.quantum-1)/b.quantum
	expectedEndTime := b.createTime.Add(b.fillInterval * time.Duration(endTick))
	waitTime := expectedEndTime.Sub(now)
	if waitTime <= maxWait {
		b.availableTokens = newAvailable
		return waitTime, true
	}
	return
}
