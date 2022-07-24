package rate_limit

import (
	"sync"
	"testing"
	"time"
)

type mockClock struct {
	d time.Duration
	t *time.Time
}

func (m *mockClock) mockNow(d time.Duration) {
	m.d = d
}
func (m *mockClock) initTime() {
	n := time.Now()
	m.t = &n
}
func (m *mockClock) Now() time.Time {
	if m.t != nil {
		return m.t.Add(m.d)
	}
	return time.Now().Add(m.d)
}

func (m *mockClock) Sleep(duration time.Duration) {

}
func testBucket(t *testing.T, c, q, count int64, d time.Duration) {
	t.Log("---------", c, q, d/time.Second)
	bucket := NewBucket(c, q, d, WithClock(new(mockClock)))
	t.Log(time.Now().Second())
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			bucket.Take(count)
			t.Log(i, "get", time.Now().Second())
		}()
	}
	wg.Wait()
	t.Log("---------")
}

func TestTakeNormal(t *testing.T) {
	testBucket(t, 1, 1, 1, time.Second)
	testBucket(t, 1, 10, 1, time.Second)
	testBucket(t, 10, 1, 10, time.Second)

}

//func TestTryTake(t *testing.T) {
//	mc := new(mockClock)
//	interval := 1 * time.Second
//	bucket := NewBucket(1, 1, interval, WithClock(mc))
//	mc.mockNow(1 * interval)
//	t.Log(bucket.TryTake(1))
//
//	mc.mockNow(1 * interval)
//	t.Log(bucket.TryTake(2))
//
//	mc.mockNow(2 * interval)
//	t.Log(bucket.TryTake(2))
//}

func TestTakeAvailable(t *testing.T) {
	mc := new(mockClock)
	interval := 1 * time.Second
	bucket := NewBucket(10, 1, interval, WithClock(mc))

	t.Log(bucket.TakeAvailable(10))

	//mc.mockNow(interval)
	//t.Log(bucket.TakeAvailable(10))
	for i := 0; i < 10; i++ {
		mc.mockNow(time.Duration(i) * interval)
		t.Log(bucket.TakeAvailable(int64(i)))
	}
}

func TestWait(t *testing.T) {
	mc := new(mockClock)
	interval := 1 * time.Second
	bucket := NewBucket(10, 1, interval, WithClock(mc))
	t.Log(bucket.TryTake(10, 9*interval))
	mc.mockNow(10 * time.Second)
	t.Log(bucket.TryTake(10, 11*interval))
}

func clear(b *TokenBucket) {
	b.availableTokens = 0
	b.lastTick = 0
}

func TestAvailable(t *testing.T) {
	mClock := new(mockClock)
	interval := time.Second
	buc := NewBucket(100, 1, interval, WithClock(mClock))

	testAvailable := func(mockDur, targetTokens int64) {
		mClock.mockNow(time.Duration(mockDur) * interval)
		res := buc.Available()

		if res != targetTokens {
			t.Fatalf("fail mockDur :%d, targetTokens:%d ,result :%d", mockDur, targetTokens, res)
		}

		clear(buc)
	}

	testAvailable(100, 100)
	testAvailable(99, 99)
	testAvailable(0, 0)
	testAvailable(0, 0)
}

func TestTick(t *testing.T) {
	mClock := new(mockClock)
	interval := time.Second
	buc := NewBucket(100, 1, interval, WithClock(mClock))
	testTick := func(d, target int64) {
		mClock.mockNow(time.Duration(d) * interval)
		ct := buc.currentTick(mClock.Now())
		if ct != target {
			t.Fatalf("test tick fail :d :%d,target :%d ,result :%d", d, target, ct)
		}
	}
	for i := int64(0); i < 100; i++ {
		testTick(i, i)
	}
}

func TestTake(t *testing.T) {
	mClock := new(mockClock)
	//mClock.initTime()

	interval := time.Second
	buc := NewBucket(100, 1, interval, WithClock(mClock))

	mClock.mockNow(0 * interval)
	w, _ := buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)

	mClock.mockNow(100 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)

	mClock.mockNow(200 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)

	mClock.mockNow(400 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)

	mClock.mockNow(500 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)

	mClock.mockNow(700 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w)
	mClock.mockNow(1000 * interval)
	w, _ = buc.take(100, mClock.Now(), infinityDuration)
	t.Log(w, buc.Available())

	mClock.mockNow(1100 * interval)
	t.Log(buc.Available())

	w, _ = buc.take(101, mClock.Now(), infinityDuration)
	t.Log(w, buc.Available())

	mClock.mockNow(1202 * interval)
	t.Log(buc.Available())
	w, _ = buc.take(102, mClock.Now(), infinityDuration)
	t.Log(w, buc.Available())

	clear(buc)
	mClock.mockNow(0)
	w, succ := buc.take(100, mClock.Now(), 15*time.Second)
	t.Log(w, succ)

	w, succ = buc.take(100, mClock.Now(), 100*time.Second)
	t.Log(w, succ)

	w, succ = buc.take(100, mClock.Now(), -1*time.Second)
	t.Log(w, succ)
}
