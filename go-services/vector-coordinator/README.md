# Vector Coordinator Service

向量化協調服務，負責根據 TraceID 協調向量化流程，調用 Python API 生成 embedding 並存儲結果。

## 功能概述

- 根據 TraceID 查詢待向量化論文
- 調用 Python embedding API
- 向量結果批次存儲到 DynamoDB
- 錯誤處理和重試機制
- 向量化進度追蹤

## 技術規格

- **語言**: Go 1.23+
- **觸發方式**: Step Function
- **記憶體**: 512MB
- **超時**: 15分鐘
- **並發處理**: 支援 (可配置)

## 專案結構

```
vector-coordinator/
├── main.go                      # Lambda 入口點
├── main_test.go                # 主要邏輯測試
├── Makefile                    # 建置和部署腳本
├── go.mod                      # Go 模組定義
├── go.sum                      # 依賴版本鎖定
├── retriever/                  # 資料檢索
│   ├── retriever.go            # DynamoDB 查詢實作
│   └── retriever_test.go       # 檢索測試
├── client/                     # API 客戶端
│   ├── vector_client.go        # 向量化 API 客戶端
│   └── vector_client_test.go   # 客戶端測試
├── storage/                    # 向量存儲
│   ├── vector_storage.go       # 向量存儲實作
│   └── vector_storage_test.go  # 存儲測試
├── build/                      # 建置輸出目錄
└── dist/                       # 部署套件目錄
```

## API 介面

### Lambda 事件格式

**Step Function 輸入**:
```json
{
  "trace_id": "batch-20240101-abc123",
  "batch_size": 100,
  "concurrent_workers": 5,
  "vector_type": "title_abstract"
}
```

**處理結果輸出**:
```json
{
  "trace_id": "batch-20240101-abc123",
  "vectorized_count": 833,
  "failed_count": 0,
  "skipped_count": 23,
  "processing_time_ms": 120000,
  "status": "completed",
  "vector_statistics": {
    "total_papers": 856,
    "successful_vectors": 833,
    "failed_vectors": 0,
    "avg_vector_time_ms": 150,
    "model_version": "v1.0"
  },
  "errors": []
}
```

### DynamoDB 查詢

**Papers Table GSI 查詢**:
```go
// 根據 trace_id 查詢論文
input := &dynamodb.QueryInput{
    TableName:              aws.String("papers-table"),
    IndexName:              aws.String("trace-id-index"),
    KeyConditionExpression: aws.String("trace_id = :trace_id"),
    ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
        ":trace_id": {S: aws.String(traceID)},
    },
}
```

**Vectors Table 寫入格式**:
```json
{
  "paper_id": "arxiv:2401.12345",
  "vector_type": "title_abstract",
  "embedding": [0.1234, -0.5678, 0.9012, ...],
  "embedding_metadata": {
    "model_name": "all-MiniLM-L6-v2",
    "model_version": "v1.0",
    "dimension": 384,
    "text_length": 256,
    "preprocessing": "lowercase_tokenize"
  },
  "source_text": {
    "content": "Deep Learning for Natural Language Processing. This paper presents...",
    "source_fields": ["title", "abstract"],
    "language": "en"
  },
  "processing_info": {
    "created_at": "2024-01-01T12:30:00Z",
    "trace_id": "batch-20240101-abc123",
    "processing_time_ms": 150
  }
}
```

## 核心功能

### 1. 資料檢索器

**功能**:
- 根據 TraceID 查詢論文資料
- 支援分頁查詢大量資料
- 過濾已向量化的論文
- 組合標題和摘要文字

**實作範例**:
```go
type Retriever struct {
    client    *dynamodb.DynamoDB
    tableName string
    indexName string
}

func (r *Retriever) GetPapersByTraceID(ctx context.Context, traceID string) ([]Paper, error) {
    var papers []Paper
    var lastEvaluatedKey map[string]*dynamodb.AttributeValue
    
    for {
        input := &dynamodb.QueryInput{
            TableName:              aws.String(r.tableName),
            IndexName:              aws.String(r.indexName),
            KeyConditionExpression: aws.String("trace_id = :trace_id"),
            ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
                ":trace_id": {S: aws.String(traceID)},
            },
            ExclusiveStartKey: lastEvaluatedKey,
        }
        
        result, err := r.client.Query(input)
        if err != nil {
            return nil, err
        }
        
        // 解析結果並添加到 papers
        batch, err := r.parsePapers(result.Items)
        if err != nil {
            return nil, err
        }
        papers = append(papers, batch...)
        
        // 檢查是否還有更多資料
        if result.LastEvaluatedKey == nil {
            break
        }
        lastEvaluatedKey = result.LastEvaluatedKey
    }
    
    return papers, nil
}
```

### 2. 向量化 API 客戶端

**功能**:
- HTTP 客戶端調用 Python API
- 請求重試和錯誤處理
- 連線池管理
- 回應驗證

**實作範例**:
```go
type VectorClient struct {
    httpClient *http.Client
    baseURL    string
    timeout    time.Duration
}

func (c *VectorClient) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResponse, error) {
    request := EmbeddingRequest{
        Text:         text,
        ModelVersion: "v1.0",
    }
    
    jsonData, err := json.Marshal(request)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embed", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %d", resp.StatusCode)
    }
    
    var response EmbeddingResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, err
    }
    
    return &response, nil
}
```

### 3. 向量存儲器

**功能**:
- 批次寫入向量到 DynamoDB
- 重複檢查和更新
- 錯誤處理和重試
- 存儲統計追蹤

**批次寫入策略**:
```go
func (s *VectorStorage) BatchStore(ctx context.Context, vectors []VectorRecord) error {
    batches := s.createBatches(vectors, 25)
    
    for i, batch := range batches {
        if err := s.storeBatch(ctx, batch); err != nil {
            return fmt.Errorf("failed to store batch %d: %w", i, err)
        }
    }
    
    return nil
}

func (s *VectorStorage) storeBatch(ctx context.Context, vectors []VectorRecord) error {
    writeRequests := make([]*dynamodb.WriteRequest, len(vectors))
    
    for i, vector := range vectors {
        item, err := vector.ToDynamoDBItem()
        if err != nil {
            return err
        }
        
        writeRequests[i] = &dynamodb.WriteRequest{
            PutRequest: &dynamodb.PutRequest{Item: item},
        }
    }
    
    _, err := s.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
        RequestItems: map[string][]*dynamodb.WriteRequest{
            s.tableName: writeRequests,
        },
    })
    
    return err
}
```

## 開發指南

### 本地開發

1. **安裝依賴**:
   ```bash
   go mod download
   ```

2. **設定環境變數**:
   ```bash
   export AWS_REGION=us-east-1
   export PAPERS_TABLE_NAME=papers-table
   export VECTORS_TABLE_NAME=vectors-table
   export EMBEDDING_API_URL=https://your-api-gateway-url
   export LOG_LEVEL=DEBUG
   ```

3. **執行測試**:
   ```bash
   make test
   ```

4. **本地建置**:
   ```bash
   make build-local
   ```

### 建置和部署

1. **建置 Lambda 套件**:
   ```bash
   make build
   ```

2. **建立部署套件**:
   ```bash
   make package
   ```

3. **驗證套件**:
   ```bash
   make verify-package
   ```

4. **部署到 AWS**:
   ```bash
   make deploy
   ```

### 測試策略

#### 單元測試
```bash
# 測試資料檢索
go test ./retriever -v

# 測試 API 客戶端
go test ./client -v

# 測試向量存儲
go test ./storage -v
```

#### 整合測試
```bash
# 啟動測試環境
docker-compose up -d localstack embedding-api

# 設定測試環境變數
export AWS_ENDPOINT_URL=http://localhost:4566
export EMBEDDING_API_URL=http://localhost:8000

# 執行整合測試
go test -tags=integration ./...
```

#### API 測試
```bash
# 測試 embedding API 連線
curl -X POST http://localhost:8000/embed \
  -H "Content-Type: application/json" \
  -d '{"text": "test embedding"}'

# 測試健康檢查
curl http://localhost:8000/health
```

## 監控和日誌

### 結構化日誌範例

```json
{
  "timestamp": "2024-01-01T12:30:00Z",
  "level": "INFO",
  "message": "Vector processing completed",
  "service": "vector-coordinator",
  "trace_id": "batch-20240101-abc123",
  "request_id": "req-456def",
  "metadata": {
    "vectorized_count": 833,
    "failed_count": 0,
    "processing_time_ms": 120000,
    "vector_statistics": {
      "total_papers": 856,
      "successful_vectors": 833,
      "avg_vector_time_ms": 150,
      "model_version": "v1.0"
    }
  }
}
```

### 關鍵指標

- **向量化成功率**: 成功向量化數 / 總論文數
- **API 回應時間**: embedding API 平均回應時間
- **並發效率**: 並行處理的效能提升
- **存儲延遲**: DynamoDB 寫入平均時間
- **錯誤率**: 失敗向量化數 / 總嘗試數

### CloudWatch 查詢

```bash
# 查看向量化統計
aws logs insights start-query \
  --log-group-name /aws/lambda/vector-coordinator \
  --start-time $(date -d '1 day ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, metadata.vectorized_count, metadata.processing_time_ms, metadata.vector_statistics.avg_vector_time_ms | filter message = "Vector processing completed" | sort @timestamp desc'

# 查看 API 錯誤
aws logs filter-log-events \
  --log-group-name /aws/lambda/vector-coordinator \
  --filter-pattern "embedding.*ERROR"
```

## 故障排除

### 常見問題

1. **Embedding API 連線失敗**:
   ```bash
   # 測試 API 連線
   curl -X GET https://your-api-gateway-url/health
   
   # 檢查 VPC 配置
   aws lambda get-function-configuration --function-name vector-coordinator
   ```

2. **DynamoDB 查詢超時**:
   ```bash
   # 檢查 GSI 狀態
   aws dynamodb describe-table --table-name papers-table
   
   # 調整查詢批次大小
   export QUERY_BATCH_SIZE=50
   ```

3. **記憶體不足**:
   ```bash
   # 調整 Lambda 記憶體
   aws lambda update-function-configuration \
     --function-name vector-coordinator \
     --memory-size 1024
   ```

4. **並發限制**:
   ```bash
   # 調整並發設定
   aws lambda put-provisioned-concurrency-config \
     --function-name vector-coordinator \
     --provisioned-concurrency-config AllocatedProvisionedConcurrencyUnits=10
   ```

### 效能調優

**並發處理配置**:
```go
type Config struct {
    MaxConcurrentWorkers int `json:"max_concurrent_workers"`
    BatchSize           int `json:"batch_size"`
    APITimeout          int `json:"api_timeout_seconds"`
}

func (c *Coordinator) processWithWorkers(papers []Paper) error {
    semaphore := make(chan struct{}, c.config.MaxConcurrentWorkers)
    var wg sync.WaitGroup
    
    for _, paper := range papers {
        wg.Add(1)
        go func(p Paper) {
            defer wg.Done()
            semaphore <- struct{}{}        // 取得 token
            defer func() { <-semaphore }() // 釋放 token
            
            c.processPaper(p)
        }(paper)
    }
    
    wg.Wait()
    return nil
}
```

## 擴展指南

### 支援多種向量類型

1. **新增向量類型**:
   ```go
   const (
       VectorTypeTitleAbstract = "title_abstract"
       VectorTypeFullText      = "full_text"
       VectorTypeConclusion    = "conclusion"
   )
   ```

2. **文字提取策略**:
   ```go
   func (p *Paper) ExtractText(vectorType string) string {
       switch vectorType {
       case VectorTypeTitleAbstract:
           return p.Title + " " + p.Abstract
       case VectorTypeFullText:
           return p.FullText
       case VectorTypeConclusion:
           return p.Conclusion
       default:
           return p.Title + " " + p.Abstract
       }
   }
   ```

### 效能優化

1. **連線池**: 重用 HTTP 連線
2. **快取**: 本地快取重複文字的向量
3. **批次優化**: 動態調整批次大小
4. **壓縮**: 向量資料壓縮存儲

### 監控增強

1. **分散式追蹤**: 整合 AWS X-Ray
2. **自定義指標**: 業務指標監控
3. **告警設定**: 基於成功率的告警
4. **效能分析**: 向量化效能分析