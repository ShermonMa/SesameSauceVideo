/*
 * database.go
 * 功能：数据库连接配置与初始化
 * 时间戳：2026-04-20
 */

package config

import (
	"fmt"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 是全局的数据库连接实例
var DB *gorm.DB

// InitDatabase 读取配置并初始化 MySQL 数据库连接
func InitDatabase() error {
	host := viper.GetString("database.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetString("database.port")
	if port == "" {
		port = "13306"
	}
	user := viper.GetString("database.user")
	if user == "" {
		user = "root"
	}
	password := viper.GetString("database.password")
	dbname := viper.GetString("database.dbname")
	if dbname == "" {
		dbname = "sesame"
	}
	charset := viper.GetString("database.charset")
	if charset == "" {
		charset = "utf8mb4"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		user, password, host, port, dbname, charset)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	return nil
}
