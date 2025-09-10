# Batch Processor Service

批次資料處理服務，負責處理 S3 中的壓縮資料，執行去重和 DynamoDB upsert 操作。

## 功能概述

- S3 事件觸發的自動資料處理
- 智慧資料去重機制
- DynamoDB 批次 upsert 操作
- TraceID 生成用於流程追蹤
- 結構化日誌和錯誤處理

## 技術規格

- **語言**: Go 1.23+
- **觸發方式**: S3 事件通知
- **記憶體**: 1024MB
- **超時**: 15分鐘
- **批次大小**: 25筆/批次 (DynamoDB 限制)

## 專案結構

```
batch-processor/
├── main.go                    # Lambda 入口點
├── main_test.go              # 主要邏輯測試
├── Makefile                  # 建置和部署腳本
├── go.mod                    # Go 模組定義
├── go.sum                    # 依賴版本鎖定
├── processor/                # 核心處理邏輯
│   ├── processor.go          # 批次處理器實作
│   └── processor_test.go     # 處理器測試
├── deduplicator/             # 去重邏輯
│   ├── deduplicator.go       # 去重演算法實作
│   └── deduplicator_test.go  # 去重測試
├── dynamodb/                 # DynamoDB 操作
│   ├── writer.go             # DynamoDB 寫入器
│   └── writer_test.go        # 寫入器測試
├── s3/                       # S3 下載功能
│   ├── downloader.go         # S3 下載器實作
│   └── downloader_test.go    # 下載器測試
├── build/                    # 建置輸出目錄
└── dist/                     # 部署套件目錄
```

## API 介面

### Lambda 事件格式

**S3 事件輸入**:
```json
{
  "Records": [{
    "eventVersion": "2.1",
    "eventSource": "aws:s3",
    "eventName": "ObjectCreated:Put",
    "s3": {
      "bucket": {
        "name": "pipeline-raw-data"
      },
      "object": {
        "key": "raw-data/2024-01-01/papers-20240101120000.gz",
        "size": 1024000
      }
    }
  }]
}
```

**處理結果輸出**:
```json
{
  "trace_id": "batch-20240101-abc123",
  "processed_count": 856,
  "duplicate_count": 23,
  "upsert_count": 833,
  "failed_count": 0,
  "timestamp": "2024-01-01T12:30:00Z",
  "status": "completed",
  "processing_time_ms": 120000,
  "s3_key": "raw-data/2024-01-01/papers-20240101120000.gz",
  "batch_statistics": {
    "total_batches": 34,
    "successful_batches": 34,
    "failed_batches": 0,
    "avg_batch_time_ms": 3500
  }
}
```

### DynamoDB 資料格式

**Papers Table 結構**:
```json
{
  "paper_id": "arxiv:2401.12345",
  "source": "arxiv",
  "title": "Deep Learning for Natural Language Processing",
  "abstract": "This paper presents...",
  "authors": ["John Doe", "Jane Smith"],
  "published_date": "2024-01-15T00:00:00Z",
  "categories": ["cs.CL", "cs.AI"],
  "raw_xml": "<entry>...</entry>",
  "trace_id": "batch-20240101-abc123",
  "batch_timestamp": "2024-01-01T12:30:00Z",
  "processing_status": "processed",
  "created_at": "2024-01-01T12:30:00Z",
  "updated_at": "2024-01-01T12:30:00Z"
}
```

## 核心功能

### 1. 資料去重機制

**去重策略**:
- 基於 `paper_id` 的主鍵去重
- 支援跨批次去重檢查
- 保留最新版本的論文資料
- 記錄去重統計資訊

**去重演算法**:
```go
type Deduplicator struct {
    seenIDs map[string]bool
    stats   DeduplicationStats
}

func (d *Deduplicator) Deduplicate(papers []Paper) []Paper {
    var unique []Paper
    for _, paper := range papers {
        if !d.seenIDs[paper.ID] {
            d.seenIDs[paper.ID] = true
            unique = append(unique, paper)
        } else {
            d.stats.DuplicateCount++
        }
    }
    return unique
}
```

### 2. DynamoDB 批次操作

**批次寫入策略**:
- 每批次最多 25 筆記錄 (AWS 限制)
- 使用 `BatchWriteItem` API
- 自動重試失敗的項目
- 指數退避重試機制

**Upsert 邏輯**:
```go
func (w *Writer) BatchUpsert(ctx context.Context, papers []Paper) error {
    batches := w.createBatches(papers, 25)
    
    for _, batch := range batches {
        writeRequests := make([]*dynamodb.WriteRequest, len(batch))
        for i, paper := range batch {
            writeRequests[i] = &dynamodb.WriteRequest{
                PutRequest: &dynamodb.PutRequest{
                    Item: paper.ToDynamoDBItem(),
                },
            }
        }
        
        _, err := w.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
            RequestItems: map[string][]*dynamodb.WriteRequest{
                w.tableName: writeRequests,
            },
        })
        
        if err != nil {
            return w.handleBatchError(ctx, err, batch)
        }
    }
    
    return nil
}
```

### 3. TraceID 管理

**TraceID 格式**: `batch-{YYYYMMDD}-{random}`

**用途**:
- 追蹤批次處理流程
- 關聯相關的論文記錄
- 支援向量化流程查詢
- 除錯和監控

## 開發指南

### 本地開發

1. **安裝依賴**:
   ```bash
   go mod download
   ```

2. **設定環境變數**:
   ```bash
   export AWS_REGION=us-east-1
   export DYNAMODB_TABLE_NAME=papers-table
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

3. **測試套件完整性**:
   ```bash
   make test-package
   ```

4. **部署到 AWS**:
   ```bash
   make deploy
   ```

### 測試策略

#### 單元測試
```bash
# 測試去重邏輯
go test ./deduplicator -v

# 測試 DynamoDB 寫入
go test ./dynamodb -v

# 測試 S3 下載
go test ./s3 -v

# 測試核心處理器
go test ./processor -v
```

#### 整合測試
```bash
# 使用 LocalStack 測試
docker run --rm -d -p 4566:4566 localstack/localstack

# 設定測試環境
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

# 建立測試資源
aws --endpoint-url=http://localhost:4566 dynamodb create-table \
  --table-name test-papers-table \
  --attribute-definitions AttributeName=paper_id,AttributeType=S \
  --key-schema AttributeName=paper_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# 執行整合測試
go test -tags=integration ./...
```

## 監控和日誌

### 結構化日誌範例

```json
{
  "timestamp": "2024-01-01T12:30:00Z",
  "level": "INFO",
  "message": "Batch processing completed",
  "service": "batch-processor",
  "trace_id": "batch-20240101-abc123",
  "request_id": "req-456def",
  "metadata": {
    "s3_key": "raw-data/2024-01-01/papers-20240101120000.gz",
    "processed_count": 856,
    "duplicate_count": 23,
    "upsert_count": 833,
    "processing_time_ms": 120000,
    "batch_statistics": {
      "total_batches": 34,
      "successful_batches": 34,
      "failed_batches": 0
    }
  }
}
```

### 關鍵指標

- **處理成功率**: 成功處理的批次數 / 總批次數
- **去重效率**: 去重數量 / 總記錄數
- **DynamoDB 寫入延遲**: 平均批次寫入時間
- **記憶體使用率**: Lambda 記憶體使用峰值
- **錯誤率**: 失敗記錄數 / 總記錄數

### CloudWatch 查詢

```bash
# 查看處理統計
aws logs insights start-query \
  --log-group-name /aws/lambda/batch-processor \
  --start-time $(date -d '1 day ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, metadata.processed_count, metadata.duplicate_count, metadata.processing_time_ms | filter message = "Batch processing completed" | sort @timestamp desc'

# 查看 DynamoDB 錯誤
aws logs filter-log-events \
  --log-group-name /aws/lambda/batch-processor \
  --filter-pattern "DynamoDB.*ERROR"
```

## 故障排除

### 常見問題

1. **DynamoDB 寫入失敗**:
   ```bash
   # 檢查資料表狀態
   aws dynamodb describe-table --table-name papers-table
   
   # 檢查 IAM 權限
   aws iam get-role-policy --role-name batch-processor-role --policy-name DynamoDBAccess
   ```

2. **記憶體不足**:
   ```bash
   # 調整 Lambda 記憶體
   aws lambda update-function-configuration \
     --function-name batch-processor \
     --memory-size 1536
   ```

3. **S3 下載超時**:
   ```bash
   # 檢查 S3 物件大小
   aws s3api head-object \
     --bucket pipeline-raw-data \
     --key raw-data/2024-01-01/papers-20240101120000.gz
   ```

4. **批次處理超時**:
   ```bash
   # 增加 Lambda 超時
   aws lambda update-function-configuration \
     --function-name batch-processor \
     --timeout 900
   ```

### 效能調優

**記憶體配置建議**:
- 小檔案 (<10MB): 512MB
- 中檔案 (10-50MB): 1024MB  
- 大檔案 (>50MB): 1536MB+

**批次大小調整**:
```go
// 根據記錄大小調整批次
func (w *Writer) calculateOptimalBatchSize(avgRecordSize int) int {
    if avgRecordSize > 10000 {
        return 10  // 大記錄使用小批次
    }
    return 25     // 預設批次大小
}
```

## 擴展指南

### 支援新資料格式

1. **新增解析器**:
   ```go
   type Parser interface {
       Parse(data []byte) ([]Paper, error)
   }
   
   type JSONParser struct{}
   func (p *JSONParser) Parse(data []byte) ([]Paper, error) {
       // 實作 JSON 解析邏輯
   }
   ```

2. **註冊解析器**:
   ```go
   func init() {
       RegisterParser("json", &JSONParser{})
       RegisterParser("xml", &XMLParser{})
   }
   ```

### 效能優化

1. **並行處理**: 使用 worker pool 並行處理批次
2. **記憶體優化**: 使用串流處理大檔案
3. **連線池**: 重用 DynamoDB 連線
4. **壓縮**: 支援多種壓縮格式

### 監控增強

1. **自定義指標**: 發送業務指標到 CloudWatch
2. **分散式追蹤**: 整合 AWS X-Ray
3. **告警設定**: 基於錯誤率和延遲的告警
4. **儀表板**: CloudWatch Dashboard 視覺化