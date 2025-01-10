package keylock

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleNewKeyLocker() {
	locker := NewKeyLocker()

	unlockSomeKey := locker.Lock("some-key")
	defer unlockSomeKey()

	// Output:
}

func ExampleKeyLocker_Lock() {
	keyLocker := NewKeyLocker()

	unlockSomeKey := keyLocker.Lock("some-key")
	fmt.Println("some-key is locked;")

	go func() {
		time.Sleep(time.Millisecond * 10)
		fmt.Println("unlock some-key;")
		unlockSomeKey()
	}()

	unlock := keyLocker.Lock("some-key")
	fmt.Println("some-key locked again. (only after unlocking)")

	defer unlock()
	// Output:
	// some-key is locked;
	// unlock some-key;
	// some-key locked again. (only after unlocking)
}

func TestKeyLocker(t *testing.T) {
	var (
		testFinished = make(chan struct{})
		timeout      = time.Second
	)

	defer func() { testFinished <- struct{}{} }()

	// deadline goroutine
	go func() {
		deadlineTimer := time.NewTimer(timeout)

		select {
		case <-testFinished:
			deadlineTimer.Stop()
		case <-deadlineTimer.C:
			// это может означать, что Locker работает не корректно или не происходит unlock
			panic("deadline timeout; probably all goroutines are sleeping;\nthis may mean that Locker is not working correctly or is not called unlock")
		}
	}()

	t.Run("check locked", func(t *testing.T) {
		var (
			locker = NewKeyLocker()
			unlock func()
		)

		unlock = locker.Lock("1")
		require.True(t, isLocked(locker, "1"))

		unlock()
		require.False(t, isLocked(locker, "1"))

		unlock = locker.Lock("1")

		// проверка, что все другие ключи не были заблокированы
		require.False(t, isLocked(locker, "2"))
		require.False(t, isLocked(locker, "3"))
		require.False(t, isLocked(locker, "4"))

		unlock()
	})

	t.Run("check multiple key locks", func(t *testing.T) {
		locker := NewKeyLocker()

		unlock1 := locker.Lock("1")
		unlock2 := locker.Lock("2")
		unlock3 := locker.Lock("3")

		require.True(t, isLocked(locker, "1"))
		require.True(t, isLocked(locker, "2"))
		require.True(t, isLocked(locker, "3"))

		unlock1()
		unlock2()
		unlock3()

		require.False(t, isLocked(locker, "1"))
		require.False(t, isLocked(locker, "2"))
		require.False(t, isLocked(locker, "3"))
	})

	t.Run("check deleting key locker on unlock", func(t *testing.T) {
		locker := NewKeyLocker()

		unlock := locker.Lock("1")
		// Ключ должен существовать после lock
		_, exist := locker.locks["1"]
		require.True(t, exist)

		unlock()
		// должен удалятся после unlock
		_, exist = locker.locks["1"]
		require.False(t, exist)
	})
}

func TestParallelLocks(t *testing.T) {
	var (
		locker = NewKeyLocker()
		wg     sync.WaitGroup

		countParallels = 0
	)

	const n = 10

	for i := 0; i < n; i++ {
		wg.Add(1)

		go locker.Do("key", func() {
			defer wg.Done()

			countParallels++
			defer func() { countParallels-- }()

			if countParallels != 1 {
				t.Errorf("countParallels=%d must be 1", countParallels)
			}
		})
	}

	wg.Wait()
}

func TestCorrectQueueLen(t *testing.T) {
	var (
		locker = NewKeyLocker()

		unlockFn = make(chan struct{})
		fnCalled = make(chan struct{})
		fn       = func() {
			fnCalled <- struct{}{}
			<-unlockFn
		}
	)

	const n = 10

	for i := 0; i < n; i++ {
		go locker.Do("key", fn)
	}

	time.Sleep(time.Millisecond * 10) // let more goroutines enter Do

	requireRefCount(t, locker, "key", n)

	for i := n; i > 0; i-- {
		<-fnCalled // wait for fn to be called

		requireRefCount(t, locker, "key", i)
		unlockFn <- struct{}{} // let's finish fn
	}

	requireRefCount(t, locker, "key", 0)
}

func requireRefCount(t *testing.T, keyLocker *KeyLocker, key string, expectCount int) {
	t.Helper()

	actualCount := refCount(keyLocker, key)
	require.Equalf(t, expectCount, actualCount, "expected %d count ref by key \"%s\", got %d", expectCount, key, actualCount)
}

func isLocked(kl *KeyLocker, key string) bool {
	return refCount(kl, key) > 0
}

func refCount(kl *KeyLocker, key string) int {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	if lock, ok := kl.locks[key]; ok {
		return lock.refCount
	}

	return 0
}
