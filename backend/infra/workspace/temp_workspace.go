/*
 * temp_workspace.go
 * 功能：基于操作系统临时目录的 Workspace 实现，封装 os.MkdirTemp 与释放清理逻辑
 * 时间戳：2026-04-26
 */

package workspace

import (
	"log"
	"os"

	"github.com/shermon/SesameSauce/domain"
)

// TempWorkspace 基于系统临时目录的 Workspace 端口实现
type TempWorkspace struct{}

// NewTempWorkspace 创建实例
func NewTempWorkspace() *TempWorkspace {
	return &TempWorkspace{}
}

// 编译期校验接口实现
var _ domain.Workspace = (*TempWorkspace)(nil)

// Acquire 申请一个临时目录，返回目录路径与释放函数
// release 函数内部捕获 os.RemoveAll 的错误并打日志，避免污染调用方流程
func (w *TempWorkspace) Acquire(prefix string) (string, func(), error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", nil, err
	}
	release := func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("[workspace] 释放临时目录失败 dir=%s: %v", dir, err)
		}
	}
	return dir, release, nil
}
