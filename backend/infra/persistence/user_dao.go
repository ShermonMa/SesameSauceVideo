/*
 * user_dao.go
 * 功能：用户数据访问对象，封装 GORM 的增删改查；新增批量按 ID 查询用于列表场景
 * 时间戳：2026-04-26
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// UserDAO 提供用户表的数据库操作
type UserDAO struct{}

// NewUserDAO 创建 UserDAO 实例
func NewUserDAO() *UserDAO {
	return &UserDAO{}
}

// Create 在数据库中创建新用户记录
func (dao *UserDAO) Create(user *domain.User) error {
	return config.DB.Create(user).Error
}

// FindByName 根据用户名查找用户
func (dao *UserDAO) FindByName(name string) (*domain.User, error) {
	var user domain.User
	err := config.DB.Where("name = ?", name).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户
func (dao *UserDAO) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := config.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByID 根据主键查找用户
func (dao *UserDAO) GetByID(id uint64) (*domain.User, error) {
	var user domain.User
	err := config.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IncrementWorkCount 原子递增用户作品数
func (dao *UserDAO) IncrementWorkCount(id uint64) error {
	return config.DB.Model(&domain.User{}).Where("id = ?", id).UpdateColumn("work_count", gorm.Expr("work_count + ?", 1)).Error
}

// GetByIDs 批量按 ID 查询用户，用于列表场景下聚合作者信息，避免 N+1
// 入参为空切片时直接返回空结果，不查询数据库
func (dao *UserDAO) GetByIDs(ids []uint64) ([]domain.User, error) {
	if len(ids) == 0 {
		return []domain.User{}, nil
	}
	var users []domain.User
	err := config.DB.Where("id IN ?", ids).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}
