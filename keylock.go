//nolint:wsl
package keylock

import "sync"

type KeyLocker struct {
	mu    sync.Mutex
	locks map[string]*refCountedLock
}

func NewKeyLocker() *KeyLocker {
	return &KeyLocker{
		mu:    sync.Mutex{},
		locks: make(map[string]*refCountedLock),
	}
}

// Do блокирует вызовы для идентичных ключей, до выполнения переданной функции fn
func (kl *KeyLocker) Do(key string, fn func()) {
	kl.lockKey(key)
	defer kl.unlockKey(key)

	fn()
}

// Lock блокирует вызовы для идентичных ключей, до вызова unlock
//
// возвращает функцию unlock, которую требуется вызвать для освобождения ключа
func (kl *KeyLocker) Lock(key string) (unlock func()) {
	kl.lockKey(key)
	return func() { kl.unlockKey(key) }
}

func (kl *KeyLocker) lockKey(key string) {
	kl.mu.Lock()

	locker, exist := kl.locks[key]
	if !exist {
		locker = &refCountedLock{}
		kl.locks[key] = locker
	}

	locker.refCount++
	kl.mu.Unlock()
	locker.Lock()
}

func (kl *KeyLocker) unlockKey(key string) {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	locker, exist := kl.locks[key]
	if !exist { // todo maybe panic?
		return
	}

	locker.Unlock()
	locker.refCount--

	if locker.refCount == 0 {
		delete(kl.locks, key)
	}
}

type refCountedLock struct {
	sync.Mutex
	refCount int
}
