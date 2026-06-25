/*
 * user.go
 * 功能：User 领域实体与业务规则校验
 *      2026-06-21 last_sys_msg_check 语义变更为”用户上次查看信箱的时间戳”，
 *      用于所有消息类型（私信/回复/赞/关注/系统通知）的未读气泡计数
 * 时间戳：2026-04-20
 */

package domain

import (
	"errors"
	"regexp"
	"time"
)

// User 代表平台用户领域实体，对应 users 表
type User struct {
	ID              uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name            string     `gorm:"column:name;size:64;not null;uniqueIndex:uk_name" json:"name"`
	Password        string     `gorm:"column:password;size:255;not null" json:"-"`
	Avatar          *string    `gorm:"column:avatar;size:255" json:"avatar,omitempty"`
	BackgroundImage *string    `gorm:"column:background_image;size:255" json:"background_image,omitempty"`
	Signature       *string    `gorm:"column:signature;size:255" json:"signature,omitempty"`
	Email           *string    `gorm:"column:email;size:128;uniqueIndex:uk_email" json:"email,omitempty"`
	FollowCount     int64      `gorm:"column:follow_count;not null;default:0" json:"follow_count"`
	FollowerCount   int64      `gorm:"column:follower_count;not null;default:0" json:"follower_count"`
	TotalFavorited  int64      `gorm:"column:total_favorited;not null;default:0" json:"total_favorited"`
	WorkCount       int64      `gorm:"column:work_count;not null;default:0" json:"work_count"`
	FavoriteCount   int64      `gorm:"column:favorite_count;not null;default:0" json:"favorite_count"`
	LastSysMsgCheck *time.Time `gorm:"column:last_sys_msg_check" json:"last_sys_msg_check,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName 指定 User 对应的数据库表名
func (User) TableName() string {
	return "users"
}

// ValidateRegistration 校验注册参数是否符合业务规则
func ValidateRegistration(username, password, email string) error {
	if len(username) < 2 || len(username) > 32 {
		return errors.New("用户名长度必须在 2-32 个字符之间")
	}
	if len(password) < 6 || len(password) > 32 {
		return errors.New("密码长度必须在 6-32 个字符之间")
	}
	if email != "" {
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(email) {
			return errors.New("邮箱格式不正确")
		}
	}
	return nil
}
