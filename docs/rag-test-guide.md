# RAG知识库测试指南

## 前置条件

1. Milvus服务已启动：`docker-compose up -d`
2. 后端服务已启动：`go run cmd/server/main.go`
3. RAG已在后台配置页面启用
4. 已获取管理员Token

## 获取管理员Token

```bash
curl -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@makejob.com",
    "password": "admin123456"
  }'
```

将返回的 `access_token` 保存，后续命令中替换 `YOUR_TOKEN`。

---

## 一、RAG配置管理

### 1.1 获取RAG配置

```bash
curl -X GET http://localhost:8082/api/admin/rag-configs \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 1.2 更新RAG配置

```bash
curl -X PUT http://localhost:8082/api/admin/rag-configs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "ai_rag_enabled": "true",
    "ai_rag_collection": "interview_questions",
    "ai_rag_top_k": "5",
    "ai_rag_score_threshold": "0.5",
    "ai_rag_milvus_addr": "localhost:19530",
    "ai_rag_milvus_user": "root",
    "ai_rag_milvus_password": "Milvus",
    "ai_rag_embed_api_key": "YOUR_ARK_API_KEY",
    "ai_rag_embed_model": "doubao-embedding-large-text-240915",
    "ai_rag_embed_base_url": "https://ark.cn-beijing.volces.com/api/v3"
  }'
```

### 1.3 测试RAG连接

```bash
curl -X POST http://localhost:8082/api/admin/rag-configs/test \
  -H "Authorization: Bearer YOUR_TOKEN"
```

预期返回：
```json
{
  "code": 0,
  "data": {
    "milvus_ok": true,
    "embedding_ok": true
  }
}
```

---

## 二、知识库文档管理

### 2.1 创建技术文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "tech_doc",
    "title": "Redis缓存穿透解决方案",
    "content": "缓存穿透是指查询一个一定不存在的数据，由于缓存不命中，每次都要去数据库查询。解决方案包括：1. 布隆过滤器：在缓存前加一层布隆过滤器，拦截不存在的key。2. 缓存空值：将查询结果为空的key也缓存起来，设置较短过期时间。3. 接口层校验：在接口层对参数进行合法性校验。",
    "metadata": {
      "tags": ["Redis", "缓存", "性能优化"],
      "difficulty": "medium",
      "category": "后端开发"
    }
  }'
```

### 2.2 创建面经文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "interview_exp",
    "title": "字节跳动Go后端面试经验",
    "content": "一面：Go语言基础，包括goroutine调度模型GMP、channel底层实现、GC三色标记法。二面：系统设计，如何设计高并发消息队列，考察消息持久化、消费者组、死信队列。三面：项目深挖，详细描述一个你负责的高并发项目，遇到的最大挑战是什么，如何解决的。",
    "metadata": {
      "company": "字节跳动",
      "position": "Go后端开发",
      "level": "社招",
      "difficulty": "hard"
    }
  }'
```

### 2.3 创建岗位要求文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "job_requirement",
    "title": "高级Go开发工程师任职要求",
    "content": "岗位职责：1. 负责核心业务系统的架构设计和开发；2. 优化系统性能，提升系统可用性和稳定性；3. 参与技术选型和技术方案评审。任职要求：1. 5年以上Go开发经验；2. 精通微服务架构，熟悉gRPC、Kubernetes、Docker；3. 有高并发系统设计经验，熟悉分布式缓存、消息队列；4. 良好的编码习惯和文档能力。",
    "metadata": {
      "company_type": "互联网大厂",
      "level": "高级",
      "salary_range": "30k-50k",
      "tags": ["Go", "微服务", "K8s", "高并发"]
    }
  }'
```

### 2.4 创建MySQL相关文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "tech_doc",
    "title": "MySQL索引优化最佳实践",
    "content": "MySQL索引优化要点：1. 最左前缀原则：联合索引遵循最左匹配；2. 覆盖索引：查询字段都在索引中，避免回表；3. 索引下推：MySQL 5.6+支持在存储引擎层过滤数据；4. 避免索引失效：不在索引列上使用函数、避免隐式类型转换、避免OR条件。慢查询优化：使用EXPLAIN分析执行计划，关注type、key、rows、Extra字段。",
    "metadata": {
      "tags": ["MySQL", "索引", "性能优化", "数据库"],
      "difficulty": "medium",
      "category": "数据库"
    }
  }'
```

### 2.5 批量导入文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents/batch-import \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "collection": "interview_questions",
    "doc_type": "tech_doc",
    "documents": [
      {
        "title": "Go并发编程-sync.Pool",
        "content": "sync.Pool是Go标准库提供的对象池，用于缓存临时对象以减少GC压力。核心原理：每个P有自己的本地池，优先从本地获取，本地没有再从其他P偷取。适用场景：频繁创建和销毁的临时对象，如buffer、连接等。注意事项：Pool中的对象可能被GC回收，不适合存储持久化数据。",
        "metadata": {"tags": ["Go", "并发", "sync.Pool"]}
      },
      {
        "title": "Go并发编程-sync.Map",
        "content": "sync.Map是Go 1.9引入的并发安全map，适用于读多写少场景。内部实现：使用两个map（read和dirty），读操作优先从read map获取，无锁并发。写操作需要加锁更新dirty map。适用场景：1. 读多写少；2. key相对稳定。不适用场景：写操作频繁，会导致read map和dirty map频繁同步。",
        "metadata": {"tags": ["Go", "并发", "sync.Map"]}
      }
    ]
  }'
```

---

## 三、文档同步

### 3.1 同步单个文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "ids": [1]
  }'
```

### 3.2 批量同步文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "ids": [1, 2, 3, 4, 5]
  }'
```

### 3.3 同步所有待同步文档

```bash
curl -X POST http://localhost:8082/api/admin/rag-documents/sync-all \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 四、语义检索测试

### 4.1 基础检索

```bash
curl -X GET "http://localhost:8082/api/admin/rag/search?query=缓存穿透如何解决&top_k=5" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 4.2 测试语义理解能力

**测试同义词理解**：
```bash
# 查询"缓存击穿"应该也能命中"缓存穿透"文档
curl -X GET "http://localhost:8082/api/admin/rag/search?query=缓存击穿怎么处理&top_k=3" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**测试相关概念检索**：
```bash
# 查询"Redis性能优化"应该命中Redis相关文档
curl -X GET "http://localhost:8082/api/admin/rag/search?query=Redis高并发场景下的性能优化&top_k=3" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**测试岗位匹配**：
```bash
# 查询"Go开发招聘"应该命中岗位要求文档
curl -X GET "http://localhost:8082/api/admin/rag/search?query=Go语言高级开发工程师招聘要求&top_k=3" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**测试面试经验检索**：
```bash
# 查询"字节面试"应该命中面经文档
curl -X GET "http://localhost:8082/api/admin/rag/search?query=字节跳动后端面试问什么&top_k=3" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**测试技术深度检索**：
```bash
# 查询"MySQL优化"应该命中MySQL索引优化文档
curl -X GET "http://localhost:8082/api/admin/rag/search?query=MySQL查询慢怎么优化&top_k=3" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 五、文档管理

### 5.1 获取文档列表

```bash
curl -X GET "http://localhost:8082/api/admin/rag-documents?page=1&page_size=20" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5.2 按类型筛选

```bash
# 只看技术文档
curl -X GET "http://localhost:8082/api/admin/rag-documents?doc_type=tech_doc" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 只看面经
curl -X GET "http://localhost:8082/api/admin/rag-documents?doc_type=interview_exp" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 只看待同步
curl -X GET "http://localhost:8082/api/admin/rag-documents?sync_status=pending" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5.3 获取文档详情

```bash
curl -X GET http://localhost:8082/api/admin/rag-documents/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5.4 更新文档

```bash
curl -X PUT http://localhost:8082/api/admin/rag-documents/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "title": "Redis缓存穿透解决方案（更新版）",
    "content": "更新后的内容..."
  }'
```

### 5.5 删除文档

```bash
curl -X DELETE http://localhost:8082/api/admin/rag-documents/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5.6 获取统计信息

```bash
curl -X GET "http://localhost:8082/api/admin/rag-documents/stats?collection=interview_questions" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

预期返回：
```json
{
  "code": 0,
  "data": {
    "tech_doc": 3,
    "interview_exp": 1,
    "job_requirement": 1
  }
}
```

---

## 六、题库索引管理

### 6.1 全量索引题目

```bash
curl -X POST http://localhost:8082/api/admin/rag/index-all \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{}'
```

### 6.2 增量索引指定题目

```bash
curl -X POST http://localhost:8082/api/admin/rag/index \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "question_ids": [1, 2, 3]
  }'
```

### 6.3 删除题目索引

```bash
curl -X DELETE http://localhost:8082/api/admin/rag/index \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "question_ids": [1, 2, 3]
  }'
```

---

## 七、验证检查清单

- [ ] RAG配置页面显示Milvus和Embedding连接正常
- [ ] 创建文档后同步状态变为"已同步"
- [ ] 语义检索返回相关度高的结果
- [ ] 同义词查询能命中相关文档
- [ ] 不同类型文档能被正确分类检索
- [ ] 批量导入和同步功能正常
- [ ] 文档更新后重新同步生效

---

## 八、常见问题

### Q1: 检索返回空结果
检查：
1. 文档是否已同步（sync_status = synced）
2. RAG是否已启用（ai_rag_enabled = true）
3. Milvus连接是否正常

### Q2: 同步失败
检查：
1. Embedding API Key是否配置正确
2. 网络是否能访问Ark API
3. 查看后端日志获取详细错误

### Q3: 检索结果不相关
检查：
1. 文档内容是否足够详细
2. 尝试调整top_k参数
3. 尝试不同的查询表述
