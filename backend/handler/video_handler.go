/*
 * video_handler.go
 * 功能：视频接口层，处理 HTTP 请求并调用应用服务；所有对外 video_id 使用 Hashids 字符串，ReportProgress 也走 hashids.Decode
 *      2026-06-20 LikeVideo 接口直接返回 LikeService 提供的乐观点赞数
 * 时间戳：2026-05-01
 */

package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/pkg/response"
)

// VideoHandler 提供视频相关的 HTTP 接口
type VideoHandler struct {
	videoService *application.VideoService
	likeService  *application.LikeService
}

// NewVideoHandler 创建 VideoHandler 实例
func NewVideoHandler() *VideoHandler {
	return &VideoHandler{
		videoService: application.NewVideoService(),
		likeService:  application.NewLikeService(),
	}
}

// CreateVideo 处理创建视频请求 POST /api/v1/videos
func (h *VideoHandler) CreateVideo(c *gin.Context) {
	var req application.CreateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	resp, err := h.videoService.CreateVideo(uid, req)
	if err != nil {
		if err.Error() == "上传并发数超限" {
			response.Fail(c, 1003, err.Error())
			return
		}
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, resp)
}

// ConfirmUpload 处理确认上传请求 POST /api/v1/videos/:video_id/confirm
func (h *VideoHandler) ConfirmUpload(c *gin.Context) {
	videoIDStr := c.Param("video_id")

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	var req application.ConfirmUploadRequest
	_ = c.ShouldBindJSON(&req)

	resp, err := h.videoService.ConfirmUpload(uid, videoIDStr, req)
	if err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, resp)
}

// CancelUpload 处理取消上传请求 POST /api/v1/videos/:video_id/cancel
func (h *VideoHandler) CancelUpload(c *gin.Context) {
	videoIDStr := c.Param("video_id")

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	resp, err := h.videoService.CancelUpload(uid, videoIDStr)
	if err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, resp)
}

// GetVideoStatus 处理查询视频处理状态请求 GET /api/v1/videos/:video_id/status
func (h *VideoHandler) GetVideoStatus(c *gin.Context) {
	videoIDStr := c.Param("video_id")

	resp, err := h.videoService.GetVideoStatus(videoIDStr)
	if err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, resp)
}

// GetVideo 处理视频详情请求 GET /api/v1/videos/:video_id
func (h *VideoHandler) GetVideo(c *gin.Context) {
	videoIDStr := c.Param("video_id")

	var userID uint64
	if uid, ok := c.Get("user_id"); ok {
		userID = uid.(uint64)
	}

	resp, err := h.videoService.GetVideo(videoIDStr, userID)
	if err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, resp)
}

// GetPlayUrl 处理获取播放地址请求 GET /api/v1/videos/:video_id/play
func (h *VideoHandler) GetPlayUrl(c *gin.Context) {
	videoIDStr := c.Param("video_id")

	resp, err := h.videoService.GetPlayUrl(videoIDStr)
	if err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, resp)
}

// ReportProgress 处理上报播放进度请求 POST /api/v1/videos/:video_id/progress
func (h *VideoHandler) ReportProgress(c *gin.Context) {
	videoIDStr := c.Param("video_id")
	videoID, err := hashids.Decode(videoIDStr)
	if err != nil {
		response.Fail(c, 1001, "无效的 video_id")
		return
	}

	var req application.ReportProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.videoService.ReportProgress(uid, videoID, req.Progress); err != nil {
		response.Fail(c, 1004, err.Error())
		return
	}

	response.Success(c, nil)
}

// ListCategories 处理分类列表请求 GET /api/v1/categories
func (h *VideoHandler) ListCategories(c *gin.Context) {
	resp, err := h.videoService.ListCategories()
	if err != nil {
		response.Fail(c, 5000, "获取分类失败")
		return
	}

	response.Success(c, resp)
}

// ListVideos 处理视频列表请求 GET /api/v1/videos，公开接口
// 支持参数：page、page_size、category_id、keyword、order_by
func (h *VideoHandler) ListVideos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	categoryID, _ := strconv.ParseUint(c.DefaultQuery("category_id", "0"), 10, 64)
	keyword := c.DefaultQuery("keyword", "")
	orderBy := c.DefaultQuery("order_by", "publish_time_desc")

	req := application.ListVideosRequest{
		Page:       page,
		PageSize:   pageSize,
		CategoryID: categoryID,
		Keyword:    keyword,
		OrderBy:    orderBy,
	}

	resp, err := h.videoService.ListVideos(req)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// LikeVideo 点赞/取消点赞视频 POST /api/v1/videos/:video_id/like
func (h *VideoHandler) LikeVideo(c *gin.Context) {
	videoIDStr := c.Param("video_id")
	videoID, err := hashids.Decode(videoIDStr)
	if err != nil {
		response.Fail(c, 1001, "无效的 video_id")
		return
	}

	var req application.LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	var count int64
	if req.Action == 1 {
		count, err = h.likeService.LikeVideo(uid, videoID)
	} else {
		count, err = h.likeService.UnlikeVideo(uid, videoID)
	}

	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, &application.LikeResponse{LikeCount: count})
}

// ListMyVideos 处理"我的视频"列表请求 GET /api/v1/users/me/videos，需要登录
// 返回登录用户的全部视频（含处理中/失败/已发布/已取消），用于上传后查看处理状态
func (h *VideoHandler) ListMyVideos(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.videoService.ListMyVideos(uid, page, pageSize)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// ListUserVideos 处理用户已发布视频列表请求 GET /api/v1/users/:id/videos，公开接口
// 2026-06-22 新增，用于个人主页的"投稿"Tab
func (h *VideoHandler) ListUserVideos(c *gin.Context) {
	idStr := c.Param("id")
	authorID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || authorID == 0 {
		response.Fail(c, 1001, "用户 ID 格式错误")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.videoService.ListUserVideos(authorID, page, pageSize)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}
