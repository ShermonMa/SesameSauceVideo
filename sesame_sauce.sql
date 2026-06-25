/*
 * SesameSauce 数据库初始化脚本
 * 功能：定义核心业务表结构，支持视频、用户、互动及通知体系
 * 时间戳：2026-04-20
 * 说明：基于 otherProject.sql 优化，统一 utf8mb4，扩展用户/视频字段，删除存储过程以符合DDD分层设计
 */

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- 用户表 (users)
-- 说明：存储平台注册用户基础信息及社交属性
-- ----------------------------
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '用户ID，自增主键',
  `name` varchar(64) NOT NULL COMMENT '用户名，唯一登录标识',
  `password` varchar(255) NOT NULL COMMENT '加密后的登录密码',
  `avatar` varchar(255) DEFAULT NULL COMMENT '用户头像URL',
  `background_image` varchar(255) DEFAULT NULL COMMENT '用户主页背景图URL',
  `signature` varchar(255) DEFAULT NULL COMMENT '个人简介/签名',
  `email` varchar(128) DEFAULT NULL COMMENT '用户邮箱，选填',
  `follow_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '关注数冗余字段，避免实时COUNT',
  `follower_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '粉丝数冗余字段',
  `total_favorited` bigint(20) NOT NULL DEFAULT '0' COMMENT '获赞总数冗余字段',
  `work_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '发布作品数冗余字段',
  `favorite_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '点赞作品数冗余字段',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '注册时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '信息更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT '软删除时间戳，NULL表示未删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`) USING BTREE COMMENT '用户名唯一索引',
  UNIQUE KEY `uk_email` (`email`) USING BTREE COMMENT '邮箱唯一索引',
  KEY `idx_created_at` (`created_at`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- ----------------------------
-- 视频表 (videos)
-- 说明：存储用户上传的视频元数据及互动计数
-- ----------------------------
DROP TABLE IF EXISTS `videos`;
CREATE TABLE `videos` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '视频ID，自增主键',
  `hashid` varchar(32) DEFAULT NULL COMMENT '对外 Hashids 加密 ID',
  `cover_hashid` varchar(32) NOT NULL DEFAULT '' COMMENT '封面独立随机 hashid，用于 MinIO 对象路径',
  `author_id` bigint(20) NOT NULL COMMENT '视频作者用户ID',
  `play_url` varchar(512) NOT NULL COMMENT '视频播放URL（MinIO等对象存储地址）',
  `cover_url` varchar(512) NOT NULL COMMENT '视频封面URL',
  `title` varchar(128) DEFAULT NULL COMMENT '视频标题',
  `description` text COMMENT '视频描述/简介',
  `duration` int(11) NOT NULL DEFAULT '0' COMMENT '视频时长（秒）',
  `width` int(11) DEFAULT NULL COMMENT '视频分辨率宽',
  `height` int(11) DEFAULT NULL COMMENT '视频分辨率高',
  `file_size` bigint(20) DEFAULT NULL COMMENT '视频文件大小（字节）',
  `progress` tinyint(4) NOT NULL DEFAULT '0' COMMENT '转码进度 0-100',
  `is_confirmed` tinyint(4) NOT NULL DEFAULT '0' COMMENT '0-未确认 1-已确认',
  `favorite_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '点赞数冗余字段',
  `comment_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '评论数冗余字段',
  `play_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '播放数冗余字段',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '视频状态：0-已发布 1-处理中 2-处理失败 3-已取消',
  `publish_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT '软删除时间戳',
  PRIMARY KEY (`id`),
  KEY `idx_author_id` (`author_id`) USING BTREE COMMENT '按作者查询视频列表',
  KEY `idx_publish_time` (`publish_time`) USING BTREE COMMENT '按时间倒序推送视频流',
  KEY `idx_status_publish` (`status`,`publish_time`) USING BTREE COMMENT '审核状态+发布时间复合索引，用于推荐流',
  UNIQUE KEY `uk_hashid` (`hashid`) USING BTREE,
  KEY `idx_status_created` (`status`,`created_at`) USING BTREE COMMENT '幽灵数据清理索引',
  FULLTEXT KEY `ft_title_desc` (`title`,`description`) COMMENT '标题和描述的全文检索索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='视频表';

-- ----------------------------
-- 点赞表 (likes)
-- 说明：用户与视频的点赞关系，cancel字段实现软取消
-- ----------------------------
DROP TABLE IF EXISTS `likes`;
CREATE TABLE `likes` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint(20) NOT NULL COMMENT '点赞用户ID',
  `video_id` bigint(20) NOT NULL COMMENT '被点赞的视频ID',
  `cancel` tinyint(4) NOT NULL DEFAULT '0' COMMENT '0-有效点赞 1-取消点赞',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '点赞时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '状态更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_video` (`user_id`,`video_id`) USING BTREE COMMENT '防止重复点赞',
  KEY `idx_video_id` (`video_id`) USING BTREE COMMENT '按视频查点赞列表',
  KEY `idx_user_id` (`user_id`) USING BTREE COMMENT '按用户查点赞过的视频'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='点赞表';

-- ----------------------------
-- 关注表 (follows)
-- 说明：用户之间的关注关系，cancel字段实现软取消，避免重复插入
-- ----------------------------
DROP TABLE IF EXISTS `follows`;
CREATE TABLE `follows` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint(20) NOT NULL COMMENT '发起关注的用户ID',
  `follower_id` bigint(20) NOT NULL COMMENT '被关注的用户ID',
  `cancel` tinyint(4) NOT NULL DEFAULT '0' COMMENT '0-有效关注 1-取消关注',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '关注时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '状态更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_follower` (`user_id`,`follower_id`) USING BTREE COMMENT '防止重复关注',
  KEY `idx_follower_id` (`follower_id`) USING BTREE COMMENT '按被关注者查粉丝列表'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='关注表';

-- ----------------------------
-- 评论表 (comments)
-- 说明：用户对视频的评论，支持一级评论（暂不支持楼中楼）
-- ----------------------------
DROP TABLE IF EXISTS `comments`;
CREATE TABLE `comments` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '评论ID，自增主键',
  `user_id` bigint(20) NOT NULL COMMENT '评论发布用户ID',
  `video_id` bigint(20) NOT NULL COMMENT '被评论的视频ID',
  `content` varchar(512) NOT NULL COMMENT '评论内容，支持Emoji',
  `like_count` bigint(20) NOT NULL DEFAULT '0' COMMENT '评论获赞数冗余字段',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '评论发布时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT '软删除时间戳，NULL表示未删除',
  PRIMARY KEY (`id`),
  KEY `idx_video_id_created` (`video_id`,`created_at`) USING BTREE COMMENT '视频评论列表按时间排序',
  KEY `idx_user_id` (`user_id`) USING BTREE COMMENT '按用户查评论历史'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='评论表';

-- ----------------------------
-- 消息通知表 (messages)
-- 说明：存储系统向用户发送的私信或互动通知（点赞、关注、评论）
-- ----------------------------
DROP TABLE IF EXISTS `messages`;
CREATE TABLE `messages` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '消息ID，自增主键',
  `from_user_id` bigint(20) NOT NULL COMMENT '发送者用户ID，0表示系统',
  `to_user_id` bigint(20) NOT NULL COMMENT '接收者用户ID',
  `content` varchar(512) NOT NULL COMMENT '消息内容',
  `action_type` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1-私信 2-关注通知 3-点赞通知 4-评论通知',
  `is_read` tinyint(4) NOT NULL DEFAULT '0' COMMENT '0-未读 1-已读',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '消息发送时间',
  PRIMARY KEY (`id`),
  KEY `idx_to_user_read` (`to_user_id`,`is_read`,`created_at`) USING BTREE COMMENT '收件箱：按接收者+已读状态+时间排序',
  KEY `idx_from_to` (`from_user_id`,`to_user_id`,`created_at`) USING BTREE COMMENT '私信会话查询'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='消息通知表';

-- ----------------------------
-- 视频分类表 (categories)
-- 说明：用于对视频进行标签化分类，便于推荐和搜索
-- ----------------------------
DROP TABLE IF EXISTS `categories`;
CREATE TABLE `categories` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '分类ID，自增主键',
  `name` varchar(64) NOT NULL COMMENT '分类名称，如搞笑、美食、科技',
  `description` varchar(255) DEFAULT NULL COMMENT '分类描述',
  `sort_order` int(11) NOT NULL DEFAULT '0' COMMENT '排序权重，越大越靠前',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='视频分类表';

-- ----------------------------
-- 视频-分类关联表 (video_categories)
-- 说明：多对多关联，一个视频可属于多个分类
-- ----------------------------
DROP TABLE IF EXISTS `video_categories`;
CREATE TABLE `video_categories` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `video_id` bigint(20) NOT NULL COMMENT '视频ID',
  `category_id` bigint(20) NOT NULL COMMENT '分类ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_category` (`video_id`,`category_id`) USING BTREE,
  KEY `idx_category_id` (`category_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='视频分类关联表';

-- ----------------------------
-- 播放历史表 (play_histories)
-- 说明：记录用户观看历史，用于“继续观看”或推荐去重
-- ----------------------------
DROP TABLE IF EXISTS `play_histories`;
CREATE TABLE `play_histories` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint(20) NOT NULL COMMENT '观看用户ID',
  `video_id` bigint(20) NOT NULL COMMENT '观看的视频ID',
  `progress` int(11) NOT NULL DEFAULT '0' COMMENT '观看进度（秒）',
  `played_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最近一次观看时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_video` (`user_id`,`video_id`) USING BTREE COMMENT '同一用户同一视频只保留一条记录',
  KEY `idx_user_played` (`user_id`,`played_at`) USING BTREE COMMENT '按用户查最近观看记录'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='播放历史表';

SET FOREIGN_KEY_CHECKS = 1;
