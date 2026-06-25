/*
 * user_service.go
 * 功能：用户应用服务，编排注册等业务流程
 * 时间戳：2026-04-20
 * 2026-06-22 扩展 UserProfileResponse 增加统计字段与 is_following，GetUserProfile 支持可选登录态
 */

package application

import (
	"errors"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/infra/pkg"
)

// UserService 处理用户相关的应用层业务逻辑
type UserService struct {
	userDAO       *persistence.UserDAO
	followService *FollowService
}

// NewUserService 创建 UserService 实例
func NewUserService() *UserService {
	return &UserService{
		userDAO:       persistence.NewUserDAO(),
		followService: NewFollowService(),
	}
}

// RegisterRequest 定义用户注册请求参数
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

// RegisterResponse 定义注册成功后的响应数据
type RegisterResponse struct {
	Token string `json:"token"`
}

// LoginRequest 定义用户登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 定义登录成功后的响应数据
type LoginResponse struct {
	Token string `json:"token"`
}

// Login 执行用户登录：查找用户 -> 校验密码 -> 发Token
func (s *UserService) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.userDAO.FindByName(req.Username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	if !pkg.CheckPassword(req.Password, user.Password) {
		return nil, errors.New("密码错误")
	}
	token, err := pkg.GenerateToken(user.ID, user.Name)
	if err != nil {
		return nil, errors.New("Token 生成失败")
	}
	return &LoginResponse{Token: token}, nil
}

// Register 执行用户注册：校验 -> 查重 -> 加密 -> 创建 -> 发Token
func (s *UserService) Register(req RegisterRequest) (*RegisterResponse, error) {
	// 1. 参数校验
	if err := domain.ValidateRegistration(req.Username, req.Password, req.Email); err != nil {
		return nil, err
	}

	// 2. 查重用户名
	existing, _ := s.userDAO.FindByName(req.Username)
	if existing != nil {
		return nil, errors.New("用户名已存在")
	}

	// 3. 如果有邮箱，查重邮箱
	if req.Email != "" {
		existingEmail, _ := s.userDAO.FindByEmail(req.Email)
		if existingEmail != nil {
			return nil, errors.New("邮箱已被注册")
		}
	}

	// 4. 加密密码
	hashedPwd, err := pkg.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	// 5. 创建用户
	user := &domain.User{
		Name:     req.Username,
		Password: hashedPwd,
	}
	if req.Email != "" {
		user.Email = &req.Email
	}

	if err := s.userDAO.Create(user); err != nil {
		return nil, errors.New("用户创建失败")
	}

	// 6. 生成 JWT Token
	token, err := pkg.GenerateToken(user.ID, user.Name)
	if err != nil {
		return nil, errors.New("Token 生成失败")
	}

	return &RegisterResponse{Token: token}, nil
}

// UserProfileResponse 用户资料响应
// 个人主页展示所需字段，不暴露密码等敏感信息
// 2026-06-22 扩展统计字段与关注状态，支撑个人主页美化
type UserProfileResponse struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Avatar          string `json:"avatar"`
	BackgroundImage string `json:"background_image"`
	Signature       string `json:"signature"`
	FollowCount     int64  `json:"follow_count"`
	FollowerCount   int64  `json:"follower_count"`
	TotalFavorited  int64  `json:"total_favorited"`
	WorkCount       int64  `json:"work_count"`
	FavoriteCount   int64  `json:"favorite_count"`
	CreatedAt       int64  `json:"created_at"`       // Unix 时间戳
	IsFollowing     bool   `json:"is_following"`     // 当前登录用户是否关注了该用户
}

// GetUserProfile 查询用户资料；Cache-Aside：先读缓存，miss 则回源 DB 并回填
// viewerID 为可选参数（0 表示未登录），用于判断当前访客是否已关注该用户
func (s *UserService) GetUserProfile(userID, viewerID uint64) (*UserProfileResponse, error) {
	if userID == 0 {
		return nil, errors.New("用户 ID 不能为空")
	}

	cv, hit, err := cache.GetUserProfile(userID)
	if err != nil {
		// 缓存读取失败仅记录，不影响继续查库
		cv, hit = cache.UserCacheValue{}, false
	}
	if hit {
		// 检查旧缓存是否缺少扩展字段，若缺少则回源补充
		if cv.FollowCount == 0 && cv.FollowerCount == 0 && cv.WorkCount == 0 {
			return s.userCacheToProfileWithFallback(cv, userID, viewerID)
		}
		return s.userCacheToProfile(cv, viewerID), nil
	}

	user, err := s.userDAO.GetByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	cacheVal := cache.UserCacheValue{
		ID:              user.ID,
		Name:            user.Name,
		Avatar:          derefString(user.Avatar),
		BackgroundImage: derefString(user.BackgroundImage),
		Signature:       derefString(user.Signature),
		FollowCount:     user.FollowCount,
		FollowerCount:   user.FollowerCount,
		TotalFavorited:  user.TotalFavorited,
		WorkCount:       user.WorkCount,
		FavoriteCount:   user.FavoriteCount,
		CreatedAt:       user.CreatedAt.Unix(),
	}
	_ = cache.SetUserProfile(userID, cacheVal)
	return s.userCacheToProfile(cacheVal, viewerID), nil
}

// userCacheToProfile 将缓存数据映射为对外响应，并计算关注状态
func (s *UserService) userCacheToProfile(cv cache.UserCacheValue, viewerID uint64) *UserProfileResponse {
	resp := &UserProfileResponse{
		ID:              cv.ID,
		Name:            cv.Name,
		Avatar:          cv.Avatar,
		BackgroundImage: cv.BackgroundImage,
		Signature:       cv.Signature,
		FollowCount:     cv.FollowCount,
		FollowerCount:   cv.FollowerCount,
		TotalFavorited:  cv.TotalFavorited,
		WorkCount:       cv.WorkCount,
		FavoriteCount:   cv.FavoriteCount,
		CreatedAt:       cv.CreatedAt,
	}
	// 登录用户判断是否已关注
	if viewerID > 0 && viewerID != cv.ID {
		resp.IsFollowing = s.followService.IsFollowing(viewerID, cv.ID)
	}
	return resp
}

// userCacheToProfileWithFallback 命中旧版缓存（缺少统计字段）时回源 DB 补齐
func (s *UserService) userCacheToProfileWithFallback(cv cache.UserCacheValue, userID, viewerID uint64) (*UserProfileResponse, error) {
	user, err := s.userDAO.GetByID(userID)
	if err != nil {
		// DB 回源失败则返回缓存已有数据（统计字段为 0）
		return s.userCacheToProfile(cv, viewerID), nil
	}

	// 用 DB 数据补充缺失的统计字段并回填缓存
	cacheVal := cache.UserCacheValue{
		ID:              cv.ID,
		Name:            cv.Name,
		Avatar:          cv.Avatar,
		BackgroundImage: cv.BackgroundImage,
		Signature:       cv.Signature,
		FollowCount:     user.FollowCount,
		FollowerCount:   user.FollowerCount,
		TotalFavorited:  user.TotalFavorited,
		WorkCount:       user.WorkCount,
		FavoriteCount:   user.FavoriteCount,
		CreatedAt:       user.CreatedAt.Unix(),
	}
	_ = cache.SetUserProfile(userID, cacheVal)
	return s.userCacheToProfile(cacheVal, viewerID), nil
}

// derefString 解引用字符串指针，nil 时返回空串
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
