package clog

import (
	"fmt"
	"sync"
	"time"
)

type MemoryLog struct {
	Mutex sync.RWMutex
	Lines []string
	Index int
}

func NewMemoryLog(capacity int) *MemoryLog {
	return &MemoryLog{Lines: make([]string, capacity, capacity), Index: 0}
}

func (ml *MemoryLog) Printf(format string, v ...interface{}) bool {
	now := time.Now().Format(time.StampMilli)

	ml.Mutex.Lock()
	defer ml.Mutex.Unlock()

	if ml.Index < cap(ml.Lines) {
		ml.Lines[ml.Index] = now + " " + fmt.Sprintf(format, v...)

		ml.Index++

		return true
	}
	return false
}

func (ml *MemoryLog) Each(fn func(line_no int, line string)) int {
	ml.Mutex.RLock()
	defer ml.Mutex.RUnlock()

	for i := 0; i < ml.Index; i++ {
		fn(i, ml.Lines[i])
	}

	return ml.Index
}

func (ml *MemoryLog) Length() int {
	ml.Mutex.RLock()
	defer ml.Mutex.RUnlock()

	return ml.Index
}

func (ml *MemoryLog) Capacity() int {
	return cap(ml.Lines)
}
