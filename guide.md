# 火山引擎 Doubao Embedding 集成指南

> 基于 SafeFlow 项目实践，介绍如何在 Go 项目中集成 `doubao-embedding-large-text-240915` 模型实现文本向量化与 RAG 检索。

## 1. 模型概述

| 属性 | 值 |
|---|---|
| 模型 ID | `doubao-embedding-large-text-240915` |
| 服务商 | 火山引擎 (Volcano Engine) Ark 平台 |
| 向量维度 | **4096** |
| 用途 | 文本转向量，用于语义检索、RAG、相似度匹配 |
| API 协议 | 兼容 OpenAI Embeddings API |

## 2. 项目架构概览

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  用户请求    │────>│   LLM Agent      │────>│  Milvus     │
│  (文本内容)  │     │  (Eino ReAct)    │     │  向量数据库  │
└─────────────┘     └──────┬───────────┘     └─────────────┘
                           │
                    ┌──────┴───────┐
                    │ Ark Embedding │
                    │ (doubao-emb)  │
                    └──────────────┘
```

项目使用 Eino 框架构建 ReAct Agent，Embedding 模型在两个场景中发挥作用：

1. **数据初始化**：将历史案例文本向量化后存入 Milvus
2. **运行时检索**：用户查询时实时向量化，在 Milvus 中检索相似案例（RAG）

## 3. 依赖安装

```bash
# Eino Ark Embedding 组件
go get github.com/cloudwego/eino-ext/components/embedding/ark

# Eino Ark Chat Model 组件（LLM 部分）
go get github.com/cloudwego/eino-ext/components/model/ark

# Milvus Retriever 组件
go get github.com/cloudwego/eino-ext/components/retriever/milvus2

# Milvus Go SDK（初始化数据用）
go get github.com/milvus-io/milvus-sdk-go/v2

# Milvus v2 Client（Retriever 用）
go get github.com/milvus-io/milvus/client/v2
```

**go.mod 中的关键依赖版本参考**：

```
github.com/cloudwego/eino v0.7.25
github.com/cloudwego/eino-ext/components/embedding/ark v0.1.1
github.com/cloudwego/eino-ext/components/retriever/milvus2 v0.0.0-20260122064704-d8be5ee82c09
github.com/milvus-io/milvus-sdk-go/v2 v2.4.2
github.com/milvus-io/milvus/client/v2 v2.6.1
```

## 4. 配置管理

### 4.1 环境变量

```bash
# 必填：火山引擎 API Key
ARK_API_KEY=your_api_key

# 必填：Embedding 模型 ID
ARK_EMBEDDING_MODEL=doubao-embedding-large-text-240915

# 可选：自定义 Endpoint（国内默认使用北京区域）
ARK_ENDPOINT=https://ark.cn-beijing.volces.com/api/v3

# Milvus 地址
MILVUS_ADDR=localhost:19530

# 可选：使用火山引擎 AK/SK 认证（替代 API Key）
VOLCENGINE_ACCESS_KEY=your_access_key
VOLCENGINE_SECRET_KEY=your_secret_key
```

### 4.2 Go 配置结构体

```go
type Config struct {
    ArkAPIKey           string `mapstructure:"ARK_API_KEY"`
    ArkEmbeddingModel   string `mapstructure:"ARK_EMBEDDING_MODEL"`
    ArkEndpoint         string `mapstructure:"ARK_ENDPOINT"`
    MilvusAddr          string `mapstructure:"MILVUS_ADDR"`
    VolcengineAccessKey string `mapstructure:"VOLCENGINE_ACCESS_KEY"`
    VolcengineSecretKey string `mapstructure:"VOLCENGINE_SECRET_KEY"`
}
```

### 4.3 认证方式

项目支持两种认证方式，优先级如下：

```
API Key 优先 > AK/SK 兜底
```

- 若配置了 `ARK_API_KEY`，使用 API Key 认证
- 若未配置 API Key 但配置了 `VOLCENGINE_ACCESS_KEY` + `VOLCENGINE_SECRET_KEY`，使用 AK/SK 认证

## 5. 初始化 Embedder

```go
import (
    "context"
    "strings"

    ark_embed "github.com/cloudwego/eino-ext/components/embedding/ark"
)

func newEmbedder(ctx context.Context, cfg *Config) (*ark_embed.Embedder, error) {
    embedCfg := &ark_embed.EmbeddingConfig{
        APIKey: cfg.ArkAPIKey,
        Model:  cfg.ArkEmbeddingModel, // "doubao-embedding-large-text-240915"
    }

    // 设置自定义 Endpoint
    if strings.TrimSpace(cfg.ArkEndpoint) != "" {
        embedCfg.BaseURL = cfg.ArkEndpoint
    }

    // AK/SK 兜底认证
    if strings.TrimSpace(embedCfg.APIKey) == "" &&
        strings.TrimSpace(cfg.VolcengineAccessKey) != "" &&
        strings.TrimSpace(cfg.VolcengineSecretKey) != "" {
        embedCfg.AccessKey = cfg.VolcengineAccessKey
        embedCfg.SecretKey = cfg.VolcengineSecretKey
    }

    return ark_embed.NewEmbedder(ctx, embedCfg)
}
```

## 6. Milvus 向量数据库集成

### 6.1 创建集合（Schema 定义）

**关键：向量维度必须设为 4096，与 `doubao-embedding-large-text-240915` 模型输出一致。**

```go
import (
    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
    CollectionName = "sensitive_cases"
    Dim            = 4096 // doubao-embedding-large-text-240915 输出维度
)

schema := &entity.Schema{
    CollectionName: CollectionName,
    Description:    "敏感案例库",
    Fields: []*entity.Field{
        {
            Name:       "id",
            DataType:   entity.FieldTypeInt64,
            PrimaryKey: true,
            AutoID:     true,
        },
        {
            Name:     "vector",
            DataType: entity.FieldTypeFloatVector,
            TypeParams: map[string]string{
                "dim": "4096", // 必须与 Embedding 模型维度一致
            },
        },
        {
            Name:     "content",
            DataType: entity.FieldTypeVarChar,
            TypeParams: map[string]string{
                "max_length": "2048",
            },
        },
        {
            Name:     "label",
            DataType: entity.FieldTypeVarChar,
            TypeParams: map[string]string{
                "max_length": "64",
            },
        },
    },
}

// 创建集合
err := c.CreateCollection(ctx, schema, entity.DefaultShardNumber)

// 创建 IVF_FLAT 索引（L2 距离）
idx, _ := entity.NewIndexIvfFlat(entity.L2, Dim)
err = c.CreateIndex(ctx, CollectionName, "vector", idx, false)
```

### 6.2 批量向量化并写入数据

```go
// 1. 准备文本数据
texts := []string{"示例文本1", "示例文本2", "示例文本3"}

// 2. 调用 Embedding API（批量）
embeddings, err := emb.EmbedStrings(ctx, texts)
// embeddings 类型: [][]float64，长度 = len(texts)，每个元素长度 = 4096

// 3. 转换为 float32（Milvus SDK 要求）
var vectors [][]float32
for _, v64 := range embeddings {
    v32 := make([]float32, len(v64))
    for i, f := range v64 {
        v32[i] = float32(f)
    }
    vectors = append(vectors, v32)
}

// 4. 插入 Milvus
_, err = c.Insert(ctx, CollectionName, "",
    entity.NewColumnFloatVector("vector", Dim, vectors),
    entity.NewColumnVarChar("content", contents),
    entity.NewColumnVarChar("label", labels),
)

// 5. 加载集合到内存（搜索前必须）
err = c.LoadCollection(ctx, CollectionName, false)
```

## 7. Eino Retriever 集成（运行时 RAG 检索）

这是项目的核心用法：将 Embedder 注入 Eino 的 Milvus Retriever，实现自动向量化检索。

```go
import (
    "github.com/cloudwego/eino-ext/components/retriever/milvus2"
    "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
    "github.com/milvus-io/milvus/client/v2/milvusclient"
)

// 初始化 Retriever
retrieverObj, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: cfg.MilvusAddr, // "localhost:19530"
    },
    Collection: "sensitive_cases",
    TopK:       3,                                          // 返回 Top 3 相似结果
    SearchMode: search_mode.NewApproximate(milvus2.COSINE), // 余弦相似度
    Embedding:  emb,                                        // 注入 Embedder
})
```

**调用方式**：传入文本字符串，Retriever 内部自动调用 Embedder 转向量后检索。

```go
docs, err := retrieverObj.Retrieve(ctx, "搜索关键词")
// docs: []*schema.Document，包含相似案例内容
```

## 8. 完整 ReAct Agent 构建

将 Embedding + Retriever + ChatModel 组合成 Eino ReAct Agent：

```go
// 1. 初始化 Embedder
emb, _ := ark_embed.NewEmbedder(ctx, embedCfg)

// 2. 初始化 Retriever（注入 Embedder）
retrieverObj, _ := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{...})

// 3. 定义搜索工具（封装 Retriever）
searchTool := utils.NewTool(searchInfo, func(ctx context.Context, args *SearchArgs) (string, error) {
    docs, err := retrieverObj.Retrieve(ctx, args.Keyword)
    // ...返回结果
})

// 4. 绑定工具到 ChatModel
chatModel.BindTools(toolInfos)

// 5. 构建 ReAct 图
g := compose.NewGraph[[]*schema.Message, *schema.Message]()
g.AddChatModelNode("model", toolModel)
g.AddToolsNode("tools", toolsNode)
g.AddEdge(compose.START, "model")
// Model 输出有 ToolCalls -> 走 tools 节点；否则 -> 结束
g.AddBranch("model", branch)
// 工具结果回到 Model，形成 ReAct 循环
g.AddEdge("tools", "model")

// 6. 编译并运行
runnable, _ := g.Compile(ctx)
resp, _ := runnable.Invoke(ctx, inputMessages)
```

## 9. Docker Compose 基础设施

```yaml
# Milvus 依赖 Etcd + MinIO
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.5

  minio:
    image: minio/minio:RELEASE.2023-03-20T20-16-18Z
    environment:
      MINIO_ACCESS_KEY: minioadmin
      MINIO_SECRET_KEY: minioadmin

  milvus:
    image: milvusdb/milvus:v2.3.0
    command: milvus run standalone
    ports:
      - "19530:19530"
    depends_on:
      - etcd
      - minio
```

## 10. 关键注意事项

### 维度匹配

`doubao-embedding-large-text-240915` 输出 **4096 维**向量。Milvus 集合的 `dim` 参数必须设为 `4096`，否则写入或查询会报错。

### float64 -> float32 转换

Eino Ark Embedder 返回 `[][]float64`，但 Milvus SDK 要求 `[][]float32`。写入前必须手动转换。

### 认证优先级

```
ARK_API_KEY > (VOLCENGINE_ACCESS_KEY + VOLCENGINE_SECRET_KEY)
```

建议生产环境使用 AK/SK 认证，开发环境使用 API Key。

### 搜索模式选择

项目使用 `search_mode.NewApproximate(milvus2.COSINE)`（余弦相似度）。其他选项：
- `COSINE`：适合归一化向量，推荐用于文本相似度
- `L2`：欧氏距离，创建索引时使用

### Retriever 容错

项目对 Embedder 和 Retriever 初始化做了容错处理（`log.Printf` 警告而非 `log.Fatal`），确保即使 Embedding 服务不可用，Agent 仍能正常运行（只是无法使用 RAG 检索）。

## 11. 快速复现 Checklist

- [ ] 开通火山引擎 Ark 平台，获取 API Key
- [ ] 部署 `doubao-embedding-large-text-240915` 模型，获取 Endpoint ID
- [ ] `docker-compose up -d etcd minio milvus` 启动 Milvus
- [ ] 设置环境变量 `ARK_API_KEY` 和 `ARK_EMBEDDING_MODEL`
- [ ] 运行 `cmd/init-milvus/main.go` 初始化集合和索引
- [ ] 在 Agent 中注入 Embedder 到 Retriever，即可使用 RAG 检索
