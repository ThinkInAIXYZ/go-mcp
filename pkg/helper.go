package pkg

import (
	"errors"
	"log"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"unsafe"
)

func Recover() {
	if r := recover(); r != nil {
		log.Printf("panic: %v\nstack: %s", r, debug.Stack())
	}
}

func RecoverWithFunc(f func(r any)) {
	if r := recover(); r != nil {
		f(r)
		log.Printf("panic: %v\nstack: %s", r, debug.Stack())
	}
}

func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func S2B(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

func JoinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return errors.New(strings.Join(messages, "; "))
}

func NewBoolAtomic() *atomic.Value {
	v := &atomic.Value{}
	v.Store(false)
	return v
}
