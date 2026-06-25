/*
 * hashids.go
 * 功能：基于 go-hashids 的对外 ID 加密模块，全局单例提供 Encode/Decode 能力
 * 时间戳：2026-05-01
 */

package hashids

import (
	"errors"
	"sync"

	"github.com/speps/go-hashids/v2"
)

var (
	encoder *hashids.HashID
	once    sync.Once
)

// InitEncoder 初始化全局 Hashids 编码器
func InitEncoder(salt string, minLength int) error {
	var initErr error
	once.Do(func() {
		hd := hashids.NewData()
		hd.Salt = salt
		hd.MinLength = minLength
		enc, err := hashids.NewWithData(hd)
		if err != nil {
			initErr = err
			return
		}
		encoder = enc
	})
	return initErr
}

// Encode 将内部自增 ID 加密为对外字符串
func Encode(id uint64) string {
	if encoder == nil {
		return ""
	}
	s, _ := encoder.Encode([]int{int(id)})
	return s
}

// Decode 将对外字符串解密为内部自增 ID
func Decode(hash string) (uint64, error) {
	if encoder == nil {
		return 0, errors.New("hashids encoder 未初始化")
	}
	ids, err := encoder.DecodeWithError(hash)
	if err != nil || len(ids) == 0 {
		return 0, errors.New("无效的 video_id")
	}
	return uint64(ids[0]), nil
}
