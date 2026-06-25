/*
 * video_query.go
 * 功能：视频读操作查询服务，从 video_service.go 拆分以践行读写分离；负责详情、列表、分类、播放地址查询
 *      2026-05-22 引入 Cache-Aside：视频详情/作者/分类各自独立缓存，互斥锁防击穿，空值防穿透，TTL 抖动防雪崩
 *      2026-06-22 改造 ListVideos 引入 ES 混合搜索分支，新增 listVideosWithES、assembleVideoListItems、ListUserVideos，扩展 orderBy 白名单
 */

package application

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/elasticsearch"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/infra/minio"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// resolveCoverURL 将数据库中的对象键（相对路径）解析为完整访问URL
// 若值已是完整URL（旧缓存兼容），直接透传返回
func resolveCoverURL(coverObjectKey string) string {
	if strings.HasPrefix(coverObjectKey, "http://") || strings.HasPrefix(coverObjectKey, "https://") {
		return coverObjectKey
	}
	if coverObjectKey == "" {
		return ""
	}
	return minio.BuildVideoBaseURL(minio.GetPublicBucket()) + "/" + coverObjectKey
}

// normalizePublishTime 将视频的发布时间归一化为可展示时间
// publish_time 因创建/发布阶段写入异常可能出现零值，此时回退到 created_at 避免前端显示 0001-01-01
func normalizePublishTime(publishTime, createdAt time.Time) time.Time {
	if publishTime.IsZero() || publishTime.Year() <= 1 {
		return createdAt
	}
	return publishTime
}

// GetVideo 根据视频 HashID 查询详情，包含作者、分类与当前用户进度（若登录）
// 读路径：先查 Redis 视频详情缓存 → Pipeline 查作者/分类缓存 → 拼装返回
// miss 链：互斥锁 → MySQL 回源 → 回填缓存
func (s *VideoService) GetVideo(hashid string, userID uint64) (*VideoDetailResponse, error) {
	videoID, err := hashids.Decode(hashid)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	// 1. 尝试读取视频详情缓存
	cacheResult, err := cache.GetVideoDetail(videoID)
	if err != nil {
		log.Printf("[GetVideo] 读取视频缓存失败 video=%d: %v", videoID, err)
	}
	if cacheResult.Hit && cacheResult.IsNull {
		return nil, errors.New("视频不存在")
	}

	var cv cache.VideoCacheGetResult
	var video *domain.Video
	var authorID uint64
	var categoryIDs []uint64

	if cacheResult.Hit {
		// 缓存命中：使用缓存数据
		cv = cacheResult
		authorID = cv.Value.AuthorID
		categoryIDs = cv.Value.CategoryIDs
	} else {
		// 2. 缓存 miss：尝试获取互斥锁回源
		lockVal, locked, lockErr := cache.AcquireVideoDetailLock(videoID)
		if lockErr != nil {
			log.Printf("[GetVideo] 获取缓存锁失败 video=%d: %v", videoID, lockErr)
		}

		if locked {
			defer cache.ReleaseVideoDetailLock(videoID, lockVal)

			// 双检：加锁后再次检查缓存（可能其他实例已回填）
			cv2, err := cache.GetVideoDetail(videoID)
			if err == nil && cv2.Hit && !cv2.IsNull {
				cv = cv2
				authorID = cv.Value.AuthorID
				categoryIDs = cv.Value.CategoryIDs
				goto assemble
			}
			if cv2.Hit && cv2.IsNull {
				return nil, errors.New("视频不存在")
			}

			// 回源 MySQL
			video, err = s.videoDAO.GetByID(videoID)
			if err != nil {
				cache.SetVideoDetailNull(videoID)
				return nil, errors.New("视频不存在")
			}
			if err := domain.CheckVisibility(video.Status); err != nil {
				cache.SetVideoDetailNull(videoID)
				return nil, err
			}

			// 查询分类 IDs
			categoryIDs, _ = s.videoCategoryDAO.GetCategoryIDsByVideoID(videoID)

			// 回填视频详情缓存
			cacheVal := cache.VideoCacheValue{
				ID:          video.ID,
				HashID:      video.HashID,
				Title:       "",
				Description: "",
				CoverURL:    video.CoverURL,
				PlayURL:     video.PlayURL,
				Duration:    video.Duration,
				PublishTime: video.PublishTime.Unix(),
				Status:      video.Status,
				AuthorID:    video.AuthorID,
				CategoryIDs: categoryIDs,
			}
			if video.Title != nil {
				cacheVal.Title = *video.Title
			}
			if video.Description != nil {
				cacheVal.Description = *video.Description
			}
			if video.Width != nil {
				cacheVal.Width = *video.Width
			}
			if video.Height != nil {
				cacheVal.Height = *video.Height
			}
			if video.FileSize != nil {
				cacheVal.FileSize = *video.FileSize
			}
			if err := cache.SetVideoDetail(videoID, cacheVal); err != nil {
				log.Printf("[GetVideo] 回填视频缓存失败 video=%d: %v", videoID, err)
			}

			authorID = video.AuthorID
			goto assemble
		}

		// 3. 未拿到锁：自旋等待，然后降级直查 DB
		time.Sleep(50 * time.Millisecond)
		cv2, err := cache.GetVideoDetail(videoID)
		if err == nil && cv2.Hit && !cv2.IsNull {
			cv = cv2
			authorID = cv.Value.AuthorID
			categoryIDs = cv.Value.CategoryIDs
			goto assemble
		}
		if cv2.Hit && cv2.IsNull {
			return nil, errors.New("视频不存在")
		}

		// 降级直查 DB，不回填缓存
		video, err = s.videoDAO.GetByID(videoID)
		if err != nil {
			return nil, errors.New("视频不存在")
		}
		if err := domain.CheckVisibility(video.Status); err != nil {
			return nil, err
		}
		categoryIDs, _ = s.videoCategoryDAO.GetCategoryIDsByVideoID(videoID)
		authorID = video.AuthorID
	}

assemble:
	// 4. 拼装响应
	return s.assembleVideoDetail(videoID, video, cv, authorID, categoryIDs, userID)
}

// assembleVideoDetail 根据视频缓存/DB 数据拼装完整详情响应
func (s *VideoService) assembleVideoDetail(videoID uint64, video *domain.Video, cv cache.VideoCacheGetResult, authorID uint64, categoryIDs []uint64, userID uint64) (*VideoDetailResponse, error) {
	var resp VideoDetailResponse

	if cv.Hit {
		// 缓存命中：从缓存值拼装基础字段
		v := cv.Value
		resp.ID = v.HashID
		resp.Title = v.Title
		resp.Description = v.Description
		resp.CoverURL = resolveCoverURL(v.CoverURL)
		resp.Duration = v.Duration
		resp.Width = v.Width
		resp.Height = v.Height
		resp.PublishTime = time.Unix(v.PublishTime, 0)
	} else {
		// DB 回源：从 video 实体拼装
		resp.ID = video.HashID
		if video.Title != nil {
			resp.Title = *video.Title
		}
		if video.Description != nil {
			resp.Description = *video.Description
		}
		resp.CoverURL = resolveCoverURL(video.CoverURL)
		resp.Duration = video.Duration
		if video.Width != nil {
			resp.Width = *video.Width
		}
		if video.Height != nil {
			resp.Height = *video.Height
		}
		resp.PublishTime = normalizePublishTime(video.PublishTime, video.CreatedAt)
	}

	// 计数字段：优先从 counter cache 读取（base + delta）
	resp.PlayCount = s.getCounterOrBase(videoID, cache.CounterFieldPlay)
	resp.FavoriteCount = s.getCounterOrBase(videoID, cache.CounterFieldLike)
	resp.CommentCount = s.getCounterOrBase(videoID, cache.CounterFieldComment)

	// 5. 查询作者（缓存优先）
	authorProfile, authorHit, err := cache.GetUserProfile(authorID)
	if err != nil {
		log.Printf("[GetVideo] 读取作者缓存失败 author=%d: %v", authorID, err)
	}
	if authorHit {
		resp.Author = AuthorDTO{
			ID:            authorProfile.ID,
			Name:          authorProfile.Name,
			Avatar:        authorProfile.Avatar,
			FollowCount:   0, // 计数器场景独立维护，列表场景暂用 0
			FollowerCount: 0,
		}
	} else {
		author, err := s.userDAO.GetByID(authorID)
		if err != nil {
			return nil, errors.New("作者信息获取失败")
		}
		resp.Author = AuthorDTO{
			ID:            author.ID,
			Name:          author.Name,
			FollowCount:   author.FollowCount,
			FollowerCount: author.FollowerCount,
		}
		if author.Avatar != nil {
			resp.Author.Avatar = *author.Avatar
		}
		// 回填作者缓存
		cacheAuthor := cache.UserCacheValue{
			ID:              author.ID,
			Name:            author.Name,
			Avatar:          resp.Author.Avatar,
			BackgroundImage: "",
			Signature:       "",
		}
		if author.BackgroundImage != nil {
			cacheAuthor.BackgroundImage = *author.BackgroundImage
		}
		if author.Signature != nil {
			cacheAuthor.Signature = *author.Signature
		}
		if err := cache.SetUserProfile(authorID, cacheAuthor); err != nil {
			log.Printf("[GetVideo] 回填作者缓存失败 author=%d: %v", authorID, err)
		}
	}

	// 6. 查询分类名称（缓存优先）
	if len(categoryIDs) > 0 {
		catNames := make([]string, 0, len(categoryIDs))
		for _, cid := range categoryIDs {
			cat, catHit, err := cache.GetCategory(cid)
			if err != nil {
				log.Printf("[GetVideo] 读取分类缓存失败 category=%d: %v", cid, err)
			}
			if catHit {
				catNames = append(catNames, cat.Name)
			} else {
				// 回源 DB
				cats, err := s.categoryDAO.GetByIDs([]uint64{cid})
				if err == nil && len(cats) > 0 {
					catNames = append(catNames, cats[0].Name)
					// 回填分类缓存
					cacheCat := cache.CategoryCacheValue{
						ID:        cats[0].ID,
						Name:      cats[0].Name,
						Description: "",
						SortOrder: cats[0].SortOrder,
					}
					if cats[0].Description != nil {
						cacheCat.Description = *cats[0].Description
					}
					if err := cache.SetCategory(cid, cacheCat); err != nil {
						log.Printf("[GetVideo] 回填分类缓存失败 category=%d: %v", cid, err)
					}
				}
			}
		}
		resp.Categories = catNames
	}

	// 7. 查询播放进度
	if userID > 0 {
		if progress, err := redisc.GetProgress(userID, videoID); err == nil && progress > 0 {
			resp.Progress = progress
		} else {
			if err != nil {
				log.Printf("[GetVideo] Redis 读取进度失败 user=%d video=%d: %v", userID, videoID, err)
			}
			if h, err := s.playHistoryDAO.GetByUserAndVideo(userID, videoID); err == nil {
				resp.Progress = h.Progress
			}
		}

		// 8. 查询当前用户是否点赞该视频
		resp.IsLiked = s.likeService.IsVideoLiked(userID, videoID)
	}

	return &resp, nil
}

// GetPlayUrl 校验视频状态并生成预签名播放地址，同时异步触发播放计数
func (s *VideoService) GetPlayUrl(hashid string) (*PlayURLResponse, error) {
	videoID, err := hashids.Decode(hashid)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return nil, errors.New("视频不存在")
	}
	if err := domain.CheckVisibility(video.Status); err != nil {
		return nil, err
	}

	playURL, expires, err := domain.GeneratePlayToken(video.HashID)
	if err != nil {
		return nil, errors.New("生成播放地址失败")
	}

	// 使用 Redis delta 替代内存 channel
	if err := cache.IncrDelta(cache.CounterFieldPlay, videoID); err != nil {
		log.Printf("[GetPlayUrl] 递增播放 delta 失败 video=%d: %v", videoID, err)
	}

	return &PlayURLResponse{
		PlayURL: playURL,
		Expires: expires,
	}, nil
}

// ListCategories 返回所有视频分类列表
func (s *VideoService) ListCategories() ([]CategoryDTO, error) {
	cats, err := s.categoryDAO.ListAll()
	if err != nil {
		return nil, err
	}
	dtos := make([]CategoryDTO, 0, len(cats))
	for _, c := range cats {
		dtos = append(dtos, CategoryDTO{
			ID:   c.ID,
			Name: c.Name,
		})
	}
	return dtos, nil
}

// ListVideos 编排：参数纠偏 → 分支路由（ES 关键词搜索 / MySQL 普通列表）→ 批量拉作者 → 拼装 DTO
// 公开接口，未登录也可调用
func (s *VideoService) ListVideos(req ListVideosRequest) (*ListVideosResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > MaxPageSize {
		req.PageSize = DefaultPageSize
	}
	// 扩展 order_by 白名单：支持 duration/publish_time 正反序 + play_count 正反序
	allowedOrders := map[string]bool{
		"publish_time_desc": true,
		"publish_time_asc":  true,
		"play_count_desc":   true,
		"play_count_asc":    true,
		"duration_desc":     true,
		"duration_asc":      true,
	}
	if !allowedOrders[req.OrderBy] {
		req.OrderBy = "publish_time_desc"
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len(keyword) > MaxKeywordLength {
		keyword = keyword[:MaxKeywordLength]
	}

	// 关键词非空且 ES 可用 → 走混合搜索；否则走 MySQL 原逻辑（向后兼容）
	if keyword != "" && elasticsearch.IsAvailable() {
		return s.listVideosWithES(req, keyword)
	}

	videos, total, err := s.videoDAO.ListPublic(req.Page, req.PageSize, req.CategoryID, keyword, req.OrderBy)
	if err != nil {
		return nil, errors.New("视频列表查询失败")
	}

	list, err := s.assembleVideoListItems(videos)
	if err != nil {
		return nil, err
	}

	return &ListVideosResponse{
		List:     list,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}

// listVideosWithES 混合搜索：ES 全文匹配 → 解码 ID → MySQL 排序+分页 → 拼装 DTO
func (s *VideoService) listVideosWithES(req ListVideosRequest, keyword string) (*ListVideosResponse, error) {
	// 1. 构建 ES 搜索请求，拉大批次由 MySQL 做最终分页
	var catIDStrs []string
	if req.CategoryID > 0 {
		catIDStrs = []string{hashids.Encode(req.CategoryID)}
	}

	esReq := domain.VideoSearchRequest{
		Keyword:     keyword,
		CategoryIDs: catIDStrs,
		Page:        1,    // ES 侧固定从 0 开始
		PageSize:    2000, // 拉大批次，MySQL 做最终排序与分页
	}

	esResult, err := s.searchIndexer.SearchVideos(context.Background(), esReq)
	if err != nil {
		log.Printf("[ListVideos] ES 搜索失败，降级 MySQL LIKE: %v", err)
		// 降级到 MySQL LIKE
		videos, total, err := s.videoDAO.ListPublic(req.Page, req.PageSize, req.CategoryID, keyword, req.OrderBy)
		if err != nil {
			return nil, errors.New("视频列表查询失败")
		}
		list, err := s.assembleVideoListItems(videos)
		if err != nil {
			return nil, err
		}
		return &ListVideosResponse{List: list, Page: req.Page, PageSize: req.PageSize, Total: total}, nil
	}

	// 2. 无匹配结果
	if len(esResult.VideoIDs) == 0 {
		return &ListVideosResponse{List: []VideoListItemDTO{}, Page: req.Page, PageSize: req.PageSize, Total: 0}, nil
	}

	// 3. 解码 hashids → 数值 ID
	numericIDs := make([]uint64, 0, len(esResult.VideoIDs))
	for _, hid := range esResult.VideoIDs {
		id, err := hashids.Decode(hid)
		if err != nil {
			continue
		}
		numericIDs = append(numericIDs, id)
	}
	if len(numericIDs) == 0 {
		return &ListVideosResponse{List: []VideoListItemDTO{}, Page: req.Page, PageSize: req.PageSize, Total: 0}, nil
	}

	// 4. MySQL 排序+关联数据查询
	videos, err := s.videoDAO.ListPublicByIDs(numericIDs, req.OrderBy)
	if err != nil {
		return nil, errors.New("视频列表查询失败")
	}

	// 5. 手动分页切片
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > len(videos) {
		start = len(videos)
	}
	if end > len(videos) {
		end = len(videos)
	}
	pagedVideos := videos[start:end]

	list, err := s.assembleVideoListItems(pagedVideos)
	if err != nil {
		return nil, err
	}

	return &ListVideosResponse{
		List:     list,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    esResult.Total,
	}, nil
}

// assembleVideoListItems 拼装视频列表项 DTO，批量查询作者与实时点赞数
func (s *VideoService) assembleVideoListItems(videos []domain.Video) ([]VideoListItemDTO, error) {
	if len(videos) == 0 {
		return []VideoListItemDTO{}, nil
	}

	// 批量查询作者
	authorIDSet := make(map[uint64]struct{}, len(videos))
	for _, v := range videos {
		authorIDSet[v.AuthorID] = struct{}{}
	}
	authorIDs := make([]uint64, 0, len(authorIDSet))
	for id := range authorIDSet {
		authorIDs = append(authorIDs, id)
	}

	authors, err := s.userDAO.GetByIDs(authorIDs)
	if err != nil {
		return nil, errors.New("作者信息查询失败")
	}
	authorMap := make(map[uint64]domain.User, len(authors))
	for _, u := range authors {
		authorMap[u.ID] = u
	}

	// 批量读取实时点赞数
	videoIDs := make([]uint64, 0, len(videos))
	for _, v := range videos {
		videoIDs = append(videoIDs, v.ID)
	}
	likeCountMap := s.likeService.BatchGetVideoLikeCount(videoIDs)

	list := make([]VideoListItemDTO, 0, len(videos))
	for _, v := range videos {
		item := VideoListItemDTO{
			ID:            v.HashID,
			CoverURL:      resolveCoverURL(v.CoverURL),
			Duration:      v.Duration,
			PlayCount:     v.PlayCount,
			FavoriteCount: likeCountMap[v.ID],
			PublishTime:   normalizePublishTime(v.PublishTime, v.CreatedAt),
		}
		if v.Title != nil {
			item.Title = *v.Title
		}
		if author, ok := authorMap[v.AuthorID]; ok {
			item.Author.ID = author.ID
			item.Author.Name = author.Name
			if author.Avatar != nil {
				item.Author.Avatar = *author.Avatar
			}
		} else {
			item.Author.ID = v.AuthorID
			item.Author.Name = "未知用户"
		}
		list = append(list, item)
	}

	return list, nil
}

// ListMyVideos 查询登录用户自己的全部视频（含处理中/失败/已发布/已取消），按 ID 倒序分页
// getCounterOrBase 从 counter cache 读取计数，base miss 时回源 DB
// 点赞数使用 LikeService 的实时计数器逻辑，避免异步刷盘窗口内读到旧值
func (s *VideoService) getCounterOrBase(videoID uint64, field string) int64 {
	if field == cache.CounterFieldLike {
		return s.likeService.GetVideoLikeCount(videoID)
	}

	base, delta, err := cache.GetCounter(field, videoID)
	if err != nil {
		log.Printf("[GetVideo] 读取计数失败 video=%d field=%s: %v", videoID, field, err)
	}
	if base == 0 {
		pc, fc, cc, err := s.videoDAO.GetVideoCounts(videoID)
		if err != nil {
			log.Printf("[GetVideo] 回源计数失败 video=%d field=%s: %v", videoID, field, err)
			return delta
		}
		var val int64
		switch field {
		case cache.CounterFieldPlay:
			val = pc
		case cache.CounterFieldLike:
			val = fc
		case cache.CounterFieldComment:
			val = cc
		}
		if err := cache.SetBase(field, videoID, val); err != nil {
			log.Printf("[GetVideo] 回填 base 失败 video=%d field=%s: %v", videoID, field, err)
		}
		base = val
	}
	return base + delta
}

func (s *VideoService) ListMyVideos(authorID uint64, page, pageSize int) (*MyVideosResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	videos, total, err := s.videoDAO.ListByAuthor(authorID, page, pageSize)
	if err != nil {
		return nil, errors.New("我的视频列表查询失败")
	}

	// 批量读取实时点赞数
	videoIDs := make([]uint64, 0, len(videos))
	for _, v := range videos {
		videoIDs = append(videoIDs, v.ID)
	}
	likeCountMap := s.likeService.BatchGetVideoLikeCount(videoIDs)

	list := make([]MyVideoListItemDTO, 0, len(videos))
	for _, v := range videos {
		item := MyVideoListItemDTO{
			ID:            v.HashID,
			CoverURL:      resolveCoverURL(v.CoverURL),
			Duration:      v.Duration,
			Status:        v.Status,
			Progress:      v.Progress,
			PlayCount:     v.PlayCount,
			FavoriteCount: likeCountMap[v.ID],
			CommentCount:  v.CommentCount,
			CreatedAt:     v.CreatedAt,
		}
		if v.Title != nil {
			item.Title = *v.Title
		}
		list = append(list, item)
	}

	return &MyVideosResponse{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// ListUserVideos 查询指定用户的已发布视频（公开接口），用于个人主页"投稿"Tab
// 2026-06-22 新增，支撑个人主页美化
func (s *VideoService) ListUserVideos(authorID uint64, page, pageSize int) (*ListVideosResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	videos, total, err := s.videoDAO.ListPublishedByAuthor(authorID, page, pageSize)
	if err != nil {
		return nil, errors.New("用户视频列表查询失败")
	}

	list, err := s.assembleVideoListItems(videos)
	if err != nil {
		return nil, err
	}

	return &ListVideosResponse{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}
