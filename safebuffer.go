package pirate

import (
	"bytes"
	"sync"
)

type safeBuffer struct {
	mutex sync.Mutex
	buf   *bytes.Buffer
}

func newSafeBuffer() *safeBuffer {
	return &safeBuffer{buf: &bytes.Buffer{}}
}

func (b *safeBuffer) Read(d []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.buf.Read(d)
}

func (b *safeBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.buf.String()
}

func (b *safeBuffer) Write(d []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.buf.Write(d)
}

func (b *safeBuffer) Reset() {
	b.mutex.Lock()
	b.buf.Reset()
	b.mutex.Unlock()
}
