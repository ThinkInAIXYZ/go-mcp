package pkg

import (
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"unsafe"

	"github.com/google/uuid"
)

// WaitForSignal 等待系统信号
func WaitForSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
}

// GenerateUUID 生成一个新的UUID字符串
func GenerateUUID() string {
	return uuid.New().String()
}

// Recover 创建Recover函数
func Recover() {
	if r := recover(); r != nil {
		log.Printf("panic: %v\nstack: %s", r, debug.Stack())
	}
}

// RecoverWithFunc 创建支持自定义处理的Recover函数
func RecoverWithFunc(f func(r any)) {
	if r := recover(); r != nil {
		f(r)
		log.Printf("panic: %v\nstack: %s", r, debug.Stack())
	}
}

// B2S 字节数组转字符串
func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
