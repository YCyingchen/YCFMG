// Package logx 提供轻量结构化日志，支持级别过滤与彩色终端输出。
package logx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

// 日志级别常量。
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	mu    sync.Mutex
	out   io.Writer = os.Stdout
	level           = LevelInfo
	color           = true
)

// ParseLevel 解析级别字符串。
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// SetLevel 设置全局日志级别。
func SetLevel(l Level) {
	mu.Lock()
	level = l
	mu.Unlock()
}

// SetOutput 设置输出目标与是否着色。
func SetOutput(w io.Writer, useColor bool) {
	mu.Lock()
	out = w
	color = useColor
	mu.Unlock()
}

func tagOf(l Level) string {
	switch l {
	case LevelDebug:
		return "DBG"
	case LevelWarn:
		return "WRN"
	case LevelError:
		return "ERR"
	default:
		return "INF"
	}
}

func colorOf(l Level) string {
	switch l {
	case LevelDebug:
		return "\033[90m"
	case LevelWarn:
		return "\033[33m"
	case LevelError:
		return "\033[31m"
	default:
		return "\033[36m"
	}
}

func logf(l Level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if l < level {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	if color {
		fmt.Fprintf(out, "%s %s%s\033[0m %s\n", ts, colorOf(l), tagOf(l), msg)
		return
	}
	fmt.Fprintf(out, "%s %s %s\n", ts, tagOf(l), msg)
}

// Debugf 输出调试日志。
func Debugf(format string, args ...any) { logf(LevelDebug, format, args...) }

// Infof 输出信息日志。
func Infof(format string, args ...any) { logf(LevelInfo, format, args...) }

// Warnf 输出警告日志。
func Warnf(format string, args ...any) { logf(LevelWarn, format, args...) }

// Errorf 输出错误日志。
func Errorf(format string, args ...any) { logf(LevelError, format, args...) }
