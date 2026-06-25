/*
 * comment_handler.go
 * 功能：评论接口层，处理发布评论、获取评论列表、获取楼中楼；video_id 统一为 hashid 字符串，进入接口后由 hashids.Decode 解码
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

// CommentHandler 提供评论相关的 HTTP 接口
type CommentHandler struct {
	commentService *application.CommentService
	likeService    *application.LikeService
}

// NewCommentHandler 创建 CommentHandler 实例
func NewCommentHandler() *CommentHandler {
	return &CommentHandler{
		commentService: application.NewCommentService(),
		likeService:    application.NewLikeService(),
	}
}

// PublishComment 发布评论 POST /api/v1/comment/publish
func (h *CommentHandler) PublishComment(c *gin.Context) {
	var req application.PublishCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	resp, err := h.commentService.PublishComment(uid, req)
	if err != nil {
		switch err.Error() {
		case "评论内容不能为空", "评论内容超过最大长度限制":
			response.Fail(c, 1001, err.Error())
		case "视频不存在":
			response.Fail(c, 1004, err.Error())
		case "评论层级超限，仅支持二级评论":
			response.Fail(c, 1001, err.Error())
		case "不能回复自己的评论":
			response.Fail(c, 1001, err.Error())
		default:
			response.Fail(c, 1002, err.Error())
		}
		return
	}

	response.Success(c, resp)
}

// ListComments 获取视频评论列表 GET /api/v1/comment/list
func (h *CommentHandler) ListComments(c *gin.Context) {
	videoIDStr := c.Query("video_id")
	videoID, err := hashids.Decode(videoIDStr)
	if err != nil {
		response.Fail(c, 1001, "无效的 video_id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	var userID uint64
	if uid, ok := c.Get("user_id"); ok {
		userID = uid.(uint64)
	}

	req := application.CommentListRequest{
		VideoID:  videoID,
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
	}

	resp, err := h.commentService.ListComments(req)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// ListReplies 获取楼中楼列表 GET /api/v1/comment/replies
func (h *CommentHandler) ListReplies(c *gin.Context) {
	rootIDStr := c.Query("root_id")
	rootID, err := strconv.ParseUint(rootIDStr, 10, 64)
	if err != nil {
		response.Fail(c, 1001, "无效的 root_id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	req := application.ReplyListRequest{
		RootID:   rootID,
		Page:     page,
		PageSize: pageSize,
	}

	resp, err := h.commentService.ListReplies(req)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// LikeComment 点赞/取消点赞评论 POST /api/v1/comment/like
func (h *CommentHandler) LikeComment(c *gin.Context) {
	var req application.LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	var resp *application.LikeResponse
	var err error
	if req.Action == 1 {
		resp, err = h.likeService.LikeComment(uid, uint64(req.CommentID))
	} else {
		resp, err = h.likeService.UnlikeComment(uid, uint64(req.CommentID))
	}

	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, resp)
}
