package notes

import "sync"

type memoryStores struct {
	notes map[int]string
	mu    sync.Mutex
}

func NewMemoryStores() *memoryStores {
	return &memoryStores{
		notes: make(map[int]string),
	}
}

func (ms *memoryStores) Save(id int, body string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.notes[id] = body
}

func (ms *memoryStores) Get(id int) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.notes[id]
}

func (ms *memoryStores) Delete(id int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.notes, id)
}
