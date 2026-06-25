/*
 * checkVideo.go
 * 功能：domain 层视频校验逻辑，负责可见性、所有权等纯业务规则判断
 * 时间戳：2026-04-26
 */

package domain

import "errors"

// CheckVisibility 把视频状态转换为面向外部访问者的可见性错误，已发布返回 nil
func CheckVisibility(status int8) error {
	switch status {
	case VideoStatusPublished:
		return nil
	case VideoStatusProcessing:
		return errors.New("视频处理中，请稍后刷新")
	case VideoStatusFailed:
		return errors.New("视频处理失败")
	case VideoStatusCancelled:
		return errors.New("视频已取消")
	default:
		return errors.New("视频暂不可见")
	}
}

// CheckCancellable 校验视频是否可以被取消，仅处理中状态返回 nil
func CheckCancellable(status int8) error {
	if status == VideoStatusProcessing {
		return nil
	}
	return errors.New("仅处理中的视频可取消")
}

// CheckVideoOwnership 校验指定用户是否为视频作者
func CheckVideoOwnership(video *Video, authorID uint64) error {
	if video.AuthorID != authorID {
		return errors.New("无权操作该视频")
	}
	return nil
}
