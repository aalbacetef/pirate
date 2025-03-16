package pirate

import (
	"bytes"
	"fmt"
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

	n, err := b.buf.Read(d)
	if err != nil {
		return n, fmt.Errorf("could not read from buffer: %w", err)
	}

	return n, nil
}

func (b *safeBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.buf.String()
}

func (b *safeBuffer) Write(d []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	n, err := b.buf.Write(d)
	if err != nil {
		return n, fmt.Errorf("could not write to buffer: %w", err)
	}

	return n, nil
}

func (b *safeBuffer) Reset() {
	b.mutex.Lock()
	b.buf.Reset()
	b.mutex.Unlock()
}
