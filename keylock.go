//nolint:wsl
package keylock

import "sync"

type KeyLocker[T comparable] struct {
	mu    sync.Mutex
	locks map[T]*refCountedLock
}

func NewKeyLocker[T comparable]() *KeyLocker[T] {
	return &KeyLocker[T]{
		mu:    sync.Mutex{},
		locks: make(map[T]*refCountedLock),
	}
}

// Do блокирует вызовы для идентичных ключей, до выполнения переданной функции fn
func (kl *KeyLocker[T]) Do(key T, fn func()) {
	kl.lockKey(key)
	defer kl.unlockKey(key)

	fn()
}

// Lock блокирует вызовы для идентичных ключей, до вызова unlock
//
// возвращает функцию unlock, которую требуется вызвать для освобождения ключа
func (kl *KeyLocker[T]) Lock(key T) (unlock func()) {
	kl.lockKey(key)
	return func() { kl.unlockKey(key) }
}

func (kl *KeyLocker[T]) lockKey(key T) {
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

func (kl *KeyLocker[T]) unlockKey(key T) {
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
