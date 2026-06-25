/*
 * timing.go
 * 功能：并发任务耗时统计工具，记录每个 goroutine 耗时，并对比累加总和与实际墙钟时间，
 *      用于评估 goroutine 改造的实际加速比；线程安全，可在多 goroutine 中并发调用 RecordSince。
 * 时间戳：2026-05-03
 */

package timing

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Tracker 并发任务耗时记录器，线程安全
type Tracker struct {
	mu      sync.Mutex
	records []record
	started time.Time
}

// record 单条任务耗时
type record struct {
	name     string
	duration time.Duration
}

// NewTracker 创建 Tracker 实例并记录起始墙钟时间
func NewTracker() *Tracker {
	return &Tracker{started: time.Now()}
}

// RecordSince 记录从 start 到现在的耗时，name 用于在报告中标识任务
// 推荐用法：goroutine 内 defer tracker.RecordSince("xxx", time.Now())
func (t *Tracker) RecordSince(name string, start time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, record{name: name, duration: time.Since(start)})
}

// Report 输出一行结构化日志：墙钟时间、各 goroutine 耗时累加、加速比、各任务明细
// prefix 通常用接口名或场景名（如 "CreateVideo"），便于日志检索
func (t *Tracker) Report(prefix string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	wallClock := time.Since(t.started)
	var sum time.Duration
	var details strings.Builder
	for _, r := range t.records {
		sum += r.duration
		details.WriteString(fmt.Sprintf(" | %s=%v", r.name, r.duration))
	}

	speedup := 0.0
	if wallClock > 0 {
		speedup = float64(sum) / float64(wallClock)
	}

	log.Printf("[timing][%s] 墙钟=%v 累加=%v 加速比=%.2fx 任务数=%d%s",
		prefix, wallClock, sum, speedup, len(t.records), details.String())
}
