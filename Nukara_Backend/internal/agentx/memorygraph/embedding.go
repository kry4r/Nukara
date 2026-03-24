package memorygraph

import (
	"context"
	"math"
)

// EmbeddingService 提供 embedding 生成和查询能力
type EmbeddingService interface {
	// EmbedText 将文本转为 embedding 向量
	EmbedText(ctx context.Context, text string) ([]float64, error)

	// GetEmbedding 获取指定节点的 embedding
	GetEmbedding(nodeID string) ([]float64, error)

	// BatchGetEmbeddings 批量获取多个节点的 embeddings
	BatchGetEmbeddings(nodeIDs []string) (map[string][]float64, error)
}

// CosineSimilarity 计算两个向量的余弦相似度
func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
