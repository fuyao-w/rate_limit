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
	"math"
	"sync"
	"time"
)

type Bucket struct {
	clock           Clock         // 时钟 用户可以传入自定义的时钟用于mock测试
	capacity        int64         // 桶容量
	fillInterval    time.Duration // 填充间隔
	quantum         int64         // 单次填充令牌数
	availableTokens int64         // 当前可用的令牌数
	createTime      time.Time     // 桶被创建的时间，用于计算间隔次数
	lastTick        int64         // 最新的间隔次数，当需要等待的时候计算截止时间
	mu              *sync.Mutex
}

// Clock 时钟 用户可以传入自定义的时钟用于mock测试
type Clock interface {
	Now() time.Time
	Sleep(duration time.Duration)
}

type Options struct {
	clock Clock
}

func NewBucket(capacity, quantum int64, fillInterval time.Duration, options ...OptionF) (bucket *Bucket) {
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
	clock := new(standardClock)
	bucket = &Bucket{
		clock:           clock,
		capacity:        capacity,
		fillInterval:    fillInterval,
		quantum:         quantum,
		createTime:      clock.Now(),
		mu:              new(sync.Mutex),
		availableTokens: capacity,
	}

	if opt.clock != nil {
		bucket.clock = opt.clock
	}
	return
}

const infinityDuration time.Duration = math.MaxInt64 // 一直阻塞直到获取令牌

// standardClock 普通的时钟实现
type standardClock struct{}

func (s *standardClock) Now() time.Time {
	return time.Now()
}

func (s *standardClock) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

type OptionF func(opt *Options)

func loadOptions(options ...OptionF) (opt *Options) {
	opt = new(Options)
	for _, f := range options {
		f(opt)
	}
	return
}

func WithClock(clock Clock) OptionF {
	return func(opt *Options) {
		opt.clock = clock
	}
}

// Available 当前可用的令牌数量
func (b *Bucket) Available() (available int64) {
	b.doWithLock(func() time.Duration {
		b.adjustAvailableTokens(b.currentTick(b.now()))
		available = b.availableTokens
		return 0
	})
	return
}

// doWithLock  封装上锁的函数处理
func (b *Bucket) doWithLock(f func() time.Duration) {
	var waitTime time.Duration
	func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		waitTime = f()
	}()
	b.clock.Sleep(waitTime)
}

// Take 获取一定数量的令牌，如果当前桶中令牌数量不够，则阻塞直到获取成功
func (b *Bucket) Take(count int64) {
	b.doWithLock(func() (waitTime time.Duration) {
		waitTime, _ = b.take(count, b.now(), infinityDuration)
		return waitTime
	})
	return
}

// TakeAvailable 尝试获取 count 个令牌，如果桶中令牌不够也不会阻塞，返回值告诉调用方实际上获取了多少个令牌
func (b *Bucket) TakeAvailable(count int64) (realCount int64) {
	b.doWithLock(func() time.Duration {
		realCount = b.takeAvailable(b.now(), count)
		return 0
	})
	return
}

// takeAvailable 尝试获取 count 个令牌，如果桶中令牌不够只获取当前可用的令牌数量
func (b *Bucket) takeAvailable(now time.Time, count int64) (realCount int64) {
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
func (b *Bucket) TryTake(count int64, maxWait time.Duration) (succ bool) {
	b.doWithLock(func() (waitTime time.Duration) {
		waitTime, succ = b.take(count, b.now(), maxWait)
		if succ {
			return
		}
		return 0
	})
	return
}

func (b *Bucket) now() time.Time {
	return b.clock.Now()
}

// currentTick 返回当前已经经过了多少填充间隔，用于计算桶里应该填充的令牌数量
func (b *Bucket) currentTick(now time.Time) int64 {
	return int64(now.Sub(b.createTime) / b.fillInterval)
}

// adjustAvailableTokens 调整当前桶中应该有的令牌数量
func (b *Bucket) adjustAvailableTokens(tick int64) {
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
func (b *Bucket) take(count int64, now time.Time, maxWait time.Duration) (waiTime time.Duration, succ bool) {
	if count <= 0 {
		return
	}

	tick := b.currentTick(now)
	b.adjustAvailableTokens(tick)
	newAvailable := b.availableTokens - count
	if newAvailable >= 0 {
		b.availableTokens = newAvailable
		return 0, true
	}
	endTick := tick + (-newAvailable+b.quantum-1)/b.quantum
	expectedEndTime := b.createTime.Add(b.fillInterval * time.Duration(endTick))
	waitTime := expectedEndTime.Sub(now)
	if waitTime <= maxWait {
		b.availableTokens = newAvailable
		return waitTime, true
	}
	return
}
