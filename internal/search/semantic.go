package search

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/vision"
)

// Semantic 是可选的外部语义向量适配器（OpenAI 兼容 /v1/embeddings）。
// 未配置时整个系统仍可正常使用感知哈希检索。
type Semantic struct {
	enabled  bool
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

// NewSemantic 创建适配器。
func NewSemantic(c config.Semantic) *Semantic {
	to := c.TimeoutSec
	if to <= 0 {
		to = 20
	}
	return &Semantic{
		enabled:  c.Enabled && strings.TrimSpace(c.Endpoint) != "",
		endpoint: strings.TrimSpace(c.Endpoint),
		apiKey:   c.APIKey,
		model:    c.Model,
		client:   &http.Client{Timeout: time.Duration(to) * time.Second},
	}
}

// Enabled 判断语义检索是否可用。
func (s *Semantic) Enabled() bool { return s != nil && s.enabled }

// Model 返回当前模型名。
func (s *Semantic) Model() string {
	if s == nil {
		return ""
	}
	return s.model
}

type embedRequest struct {
	Model string `json:"model"`
	Input []any  `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Semantic) url() string {
	e := strings.TrimRight(s.endpoint, "/")
	if strings.HasSuffix(e, "/embeddings") {
		return e
	}
	if strings.HasSuffix(e, "/v1") {
		return e + "/embeddings"
	}
	return e + "/v1/embeddings"
}

func (s *Semantic) embed(ctx context.Context, inputs []any) ([][]float32, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("语义检索未启用")
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: s.model, Input: inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("解析向量响应失败: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("向量服务错误: %s", out.Error.Message)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("向量服务返回状态 %d", resp.StatusCode)
	}
	sort.Slice(out.Data, func(i, j int) bool { return out.Data[i].Index < out.Data[j].Index })
	vecs := make([][]float32, 0, len(out.Data))
	for _, d := range out.Data {
		vecs = append(vecs, vision.NormalizeVector(d.Embedding))
	}
	return vecs, nil
}

// EmbedText 生成文本向量。
func (s *Semantic) EmbedText(ctx context.Context, text string) ([]float32, error) {
	vecs, err := s.embed(ctx, []any{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("向量服务未返回结果")
	}
	return vecs[0], nil
}

// EmbedImage 生成图片向量（base64 data URL）。
func (s *Semantic) EmbedImage(ctx context.Context, mime string, data []byte) ([]float32, error) {
	if mime == "" {
		mime = "image/jpeg"
	}
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	vecs, err := s.embed(ctx, []any{map[string]any{"image": dataURL}})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("向量服务未返回结果")
	}
	return vecs[0], nil
}

// VectorSearch 用给定向量在已生成的图片向量中检索。
func (s *Semantic) VectorSearch(vec []float32, st *store.Store, lib string, limit int) ([]Hit, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("语义检索未启用")
	}
	if limit <= 0 {
		limit = 60
	}
	all := st.LoadEmbeddings(s.model)
	if len(all) == 0 {
		return nil, fmt.Errorf("尚未生成图片向量")
	}
	type scored struct {
		id int64
		sc float64
	}
	list := make([]scored, 0, len(all))
	for id, v := range all {
		var dot float64
		n := len(v)
		if len(vec) < n {
			n = len(vec)
		}
		for i := 0; i < n; i++ {
			dot += float64(v[i]) * float64(vec[i])
		}
		list = append(list, scored{id: id, sc: dot})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].sc > list[j].sc })
	hits := []Hit{}
	for _, it := range list {
		if len(hits) >= limit {
			break
		}
		f, err := st.GetFileByID(it.id)
		if err != nil {
			continue
		}
		if lib != "" && f.Lib != lib {
			continue
		}
		hits = append(hits, Hit{File: *f, Score: it.sc, Reason: "语义匹配"})
	}
	return hits, nil
}
