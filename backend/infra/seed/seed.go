/*
 * seed.go
 * 功能：数据库种子数据初始化，包含默认视频分类
 * 时间戳：2026-04-26
 */

package seed

import (
	"log"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// defaultCategories 定义系统默认视频分类
var defaultCategories = []domain.Category{
	{Name: "娱乐", Description: strPtr("搞笑、综艺、明星等娱乐内容")},
	{Name: "音乐", Description: strPtr("MV、翻唱、乐器演奏等音乐内容")},
	{Name: "影视", Description: strPtr("电影、电视剧、影评等影视内容")},
	{Name: "游戏", Description: strPtr("游戏解说、攻略、直播等游戏内容")},
	{Name: "科技", Description: strPtr("数码评测、编程、AI等科技内容")},
	{Name: "美食", Description: strPtr("烹饪教程、探店、美食测评等")},
	{Name: "旅行", Description: strPtr("旅游攻略、风景记录、Vlog等")},
	{Name: "教育", Description: strPtr("学科知识、语言学习、考试等")},
	{Name: "运动", Description: strPtr("健身、球类、极限运动等")},
	{Name: "生活", Description: strPtr("日常分享、家居、养生等")},
	{Name: "时尚", Description: strPtr("穿搭、美妆、潮流等")},
	{Name: "萌宠", Description: strPtr("宠物日常、萌宠搞笑等")},
	{Name: "汽车", Description: strPtr("汽车评测、改装、驾驶等")},
	{Name: "动漫", Description: strPtr("动画、漫画、二次元等")},
	{Name: "知识", Description: strPtr("科普、人文、历史等知识内容")},
}

func strPtr(s string) *string {
	return &s
}

// SeedCategories 初始化默认视频分类，避免重复插入
func SeedCategories() {
	dao := persistence.NewCategoryDAO()
	for _, cat := range defaultCategories {
		var existing domain.Category
		result := config.DB.Where("name = ?", cat.Name).First(&existing)
		if result.Error != nil {
			// 不存在则创建
			if err := dao.Create(&cat); err != nil {
				log.Printf("种子数据插入失败 [分类: %s]: %v", cat.Name, err)
			} else {
				log.Printf("种子数据已插入: %s", cat.Name)
			}
		}
	}
}
