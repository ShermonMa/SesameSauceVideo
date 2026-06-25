/*
 * client.go
 * 功能：Elasticsearch 客户端封装，提供视频索引创建、文档增删查改能力；初始化时增加 Ping 检测，启动失败返回 error；
 *       2026-06-22 新增全文搜索、文档局部更新、ESIndexer 适配器与 mapping 幂等升级
 * 时间戳：2026-06-19
 */

package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/shermon/SesameSauce/domain"
)

var client *elasticsearch.Client

// InitES 初始化 ES 客户端，并通过 Ping 检测集群可用性；失败时返回 error
// 初始化成功后幂等升级 mapping，确保已有索引包含所有必要字段
func InitES(addresses []string) error {
	if len(addresses) == 0 {
		return fmt.Errorf("未配置 ES 地址")
	}
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}
	c, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("创建 ES 客户端失败: %w", err)
	}

	res, err := c.Ping()
	if err != nil {
		return fmt.Errorf("Ping ES 失败: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("ES Ping 响应错误: %s", res.String())
	}

	client = c
	log.Println("[es] 客户端初始化成功")

	// 幂等升级 mapping，兼容已有索引
	ensureMapping()
	return nil
}

// IsAvailable 判断 ES 是否可用
func IsAvailable() bool {
	return client != nil
}

const indexName = "video_categories"

// ensureMapping 幂等追加新字段到已有索引 mapping（不影响已有字段）
func ensureMapping() {
	if client == nil {
		return
	}
	// 先确保索引存在
	ensureIndex()

	newFields := `{
  "properties": {
    "description": { "type": "text" },
    "duration": { "type": "integer" },
    "play_count": { "type": "long" },
    "publish_time": { "type": "date" }
  }
}`
	res, err := client.Indices.PutMapping(
		[]string{indexName},
		strings.NewReader(newFields),
	)
	if err != nil {
		log.Printf("[es] 更新 mapping 失败: %v", err)
		return
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("[es] 更新 mapping 响应错误: %s", res.String())
	} else {
		log.Println("[es] mapping 已更新（幂等）")
	}
}

// ensureIndex 确保索引存在并设置完整 mapping
func ensureIndex() {
	if client == nil {
		return
	}
	res, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		log.Printf("[es] 检查索引存在性失败: %v", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return
	}

	mapping := `{
  "mappings": {
    "properties": {
      "video_id": { "type": "keyword" },
      "title": { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "description": { "type": "text" },
      "category_ids": { "type": "keyword" },
      "category_names": { "type": "keyword" },
      "author_id": { "type": "keyword" },
      "duration": { "type": "integer" },
      "play_count": { "type": "long" },
      "publish_time": { "type": "date" },
      "created_at": { "type": "date" }
    }
  }
}`
	res, err = client.Indices.Create(indexName, client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		log.Printf("[es] 创建索引失败: %v", err)
		return
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("[es] 创建索引响应错误: %s", res.String())
	} else {
		log.Println("[es] 索引创建成功")
	}
}

// IndexVideo 将视频文档索引到 ES；失败时仅打印日志，不阻塞主流程
func IndexVideo(ctx context.Context, doc domain.VideoSearchDoc) error {
	if client == nil {
		return nil
	}
	ensureIndex()
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	res, err := client.Index(indexName, bytes.NewReader(data), client.Index.WithContext(ctx), client.Index.WithDocumentID(doc.VideoID))
	if err != nil {
		log.Printf("[es] 索引文档失败 video=%s: %v", doc.VideoID, err)
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("[es] 索引文档响应错误 video=%s: %s", doc.VideoID, res.String())
		return nil
	}
	return nil
}

// DeleteVideo 从 ES 删除视频文档
func DeleteVideo(ctx context.Context, videoID string) error {
	if client == nil {
		return nil
	}
	res, err := client.Delete(indexName, videoID, client.Delete.WithContext(ctx))
	if err != nil {
		log.Printf("[es] 删除文档失败 video=%s: %v", videoID, err)
		return err
	}
	defer res.Body.Close()
	return nil
}

// SearchVideos 基于 ES 全文搜索匹配视频 ID，支持分类过滤
// 关键词使用 multi_match 查询 title^2 + description，best_fields + AND 语义
func SearchVideos(ctx context.Context, req domain.VideoSearchRequest) (*domain.VideoSearchResult, error) {
	if client == nil {
		return nil, fmt.Errorf("ES 不可用")
	}

	// 构建 bool query
	var mustClauses []map[string]interface{}
	if req.Keyword != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":    req.Keyword,
				"fields":   []string{"title^2", "description"},
				"type":     "best_fields",
				"operator": "and",
			},
		})
	}

	var filterClauses []map[string]interface{}
	if len(req.CategoryIDs) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"category_ids": req.CategoryIDs,
			},
		})
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   mustClauses,
				"filter": filterClauses,
			},
		},
		"from":              (req.Page - 1) * req.PageSize,
		"size":              req.PageSize,
		"_source":           []string{"video_id"},
		"track_total_hits":  true,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("ES 查询序列化失败: %w", err)
	}

	res, err := client.Search(
		client.Search.WithContext(ctx),
		client.Search.WithIndex(indexName),
		client.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, fmt.Errorf("ES 搜索请求失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES 搜索响应错误: %s", res.String())
	}

	var result esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ES 搜索响应解析失败: %w", err)
	}

	videoIDs := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		videoIDs = append(videoIDs, hit.Source.VideoID)
	}

	return &domain.VideoSearchResult{
		VideoIDs: videoIDs,
		Total:    result.Hits.Total.Value,
	}, nil
}

// esSearchResponse ES _search 响应解析结构体
type esSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source struct {
				VideoID string `json:"video_id"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// UpdateVideoFields 局部更新 ES 文档指定字段
func UpdateVideoFields(ctx context.Context, videoID string, fields map[string]interface{}) error {
	if client == nil {
		return nil
	}
	body := map[string]interface{}{"doc": fields}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	res, err := client.Update(indexName, videoID, bytes.NewReader(data),
		client.Update.WithContext(ctx),
	)
	if err != nil {
		log.Printf("[es] 更新文档失败 video=%s: %v", videoID, err)
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("[es] 更新文档响应错误 video=%s: %s", videoID, res.String())
	}
	return nil
}

// ESIndexer 实现 domain.SearchIndexer 接口，代理到包级函数
type ESIndexer struct{}

// IndexVideo 实现 SearchIndexer.IndexVideo
func (ESIndexer) IndexVideo(ctx context.Context, doc domain.VideoSearchDoc) error {
	return IndexVideo(ctx, doc)
}

// DeleteVideo 实现 SearchIndexer.DeleteVideo
func (ESIndexer) DeleteVideo(ctx context.Context, videoID string) error {
	return DeleteVideo(ctx, videoID)
}

// SearchVideos 实现 SearchIndexer.SearchVideos
func (ESIndexer) SearchVideos(ctx context.Context, req domain.VideoSearchRequest) (*domain.VideoSearchResult, error) {
	return SearchVideos(ctx, req)
}

// UpdateVideoFields 实现 SearchIndexer.UpdateVideoFields
func (ESIndexer) UpdateVideoFields(ctx context.Context, videoID string, fields map[string]interface{}) error {
	return UpdateVideoFields(ctx, videoID, fields)
}
