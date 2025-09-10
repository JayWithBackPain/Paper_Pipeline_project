# Data Collector Service

學術論文資料收集服務，負責從多個外部 API 收集論文資料並壓縮存儲至 S3。

## 功能概述

- 支援多個學術資料來源 (arXiv, PubMed, Semantic Scholar)
- 自動資料格式轉換和標準化
- Gzip 壓縮減少存儲成本
- 結構化日誌和錯誤處理
- 可配置的速率限制和批次大小

## 技術規格

- **語言**: Go 1.23+
- **觸發方式**: CloudWatch Events (定時執行)
- **記憶體**: 512MB
- **超時**: 15分鐘
- **並發**: 預設 1000

## 專案結構

```
data-collector/
├── main.go                 # Lambda 入口點
├── main_test.go            # 主要邏輯測試
├── Makefile               # 建置和部署腳本
├── go.mod                 # Go 模組定義
├── go.sum                 # 依賴版本鎖定
├── arxiv/                 # arXiv API 客戶端
│   ├── client.go          # API 客戶端實作
│   └── client_test.go     # 客戶端測試
├── config/                # 配置管理
│   ├── config.go          # 配置載入邏輯
│   └── config_test.go     # 配置測試
├── s3/                    # S3 上傳功能
│   ├── uploader.go        # S3 上傳實作
│   └── uploader_test.go   # 上傳測試
├── types/                 # 資料類型定義
│   ├── paper.go           # 論文資料結構
│   └── paper_test.go      # 類型測試
├── build/                 # 建置輸出目錄
└── dist/                  # 部署套件目錄
```

## API 介面

### Lambda 事件格式

**輸入事件**:
```json
{
  "source": "arxiv",
  "date_range": {
    "start": "2024-01-01",
    "end": "2024-01-02"
  },
  "max_results": 1000,
  "config_s3_key": "config/pipeline-config.yaml"
}
```

**輸出回應**:
```json
{
  "status": "success",
  "collected_count": 856,
  "s3_key": "raw-data/2024-01-01/papers-20240101120000.gz",
  "processing_time_ms": 45000,
  "source": "arxiv",
  "errors": []
}
```

### 配置格式

**YAML 配置檔案** (`config/pipeline-config.yaml`):
```yaml
data_sources:
  arxiv:
    api_endpoint: "http://export.arxiv.org/api/query"
    rate_limit: 3              # requests per second
    max_results: 1000          # per request
    timeout_seconds: 30
    fields_mapping:
      id: "id"
      title: "title"
      abstract: "summary"
      authors: "author"
      published: "published"
      categories: "category"
    
  pubmed:
    api_endpoint: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"
    rate_limit: 10
    max_results: 1000
    timeout_seconds: 30
    fields_mapping:
      id: "pmid"
      title: "ArticleTitle"
      abstract: "AbstractText"
      authors: "AuthorList"
      published: "PubDate"

s3_config:
  bucket: "pipeline-raw-data"
  prefix: "raw-data"
  compression: "gzip"
  
logging:
  level: "INFO"
  structured: true
```

## 開發指南

### 本地開發

1. **安裝依賴**:
   ```bash
   go mod download
   ```

2. **執行測試**:
   ```bash
   make test
   ```

3. **本地建置**:
   ```bash
   make build-local
   ```

4. **本地執行**:
   ```bash
   make local-run
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

### 測試

#### 單元測試
```bash
# 執行所有測試
go test ./...

# 執行特定套件測試
go test ./arxiv
go test ./config
go test ./s3

# 執行測試並顯示覆蓋率
go test -cover ./...
```

#### 整合測試
```bash
# 使用 LocalStack 進行整合測試
docker run --rm -it -p 4566:4566 localstack/localstack

# 設定測試環境變數
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

# 執行整合測試
go test -tags=integration ./...
```

## 監控和日誌

### 結構化日誌格式

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "INFO",
  "message": "Data collection completed",
  "service": "data-collector",
  "trace_id": "collect-20240101-abc123",
  "request_id": "req-456def",
  "metadata": {
    "source": "arxiv",
    "collected_count": 856,
    "processing_time_ms": 45000,
    "s3_key": "raw-data/2024-01-01/papers-20240101120000.gz"
  }
}
```

### 關鍵指標

- **收集成功率**: 成功收集的資料筆數 / 總請求筆數
- **API 回應時間**: 外部 API 平均回應時間
- **壓縮比率**: 壓縮後檔案大小 / 原始資料大小
- **S3 上傳時間**: 檔案上傳到 S3 的時間
- **錯誤率**: 失敗請求數 / 總請求數

### CloudWatch 查詢

```bash
# 查看收集統計
aws logs insights start-query \
  --log-group-name /aws/lambda/data-collector \
  --start-time $(date -d '1 day ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, metadata.collected_count, metadata.processing_time_ms | filter message = "Data collection completed" | sort @timestamp desc'

# 查看錯誤日誌
aws logs filter-log-events \
  --log-group-name /aws/lambda/data-collector \
  --filter-pattern "ERROR"
```

## 故障排除

### 常見問題

1. **API 速率限制**:
   - 檢查配置中的 `rate_limit` 設定
   - 調整請求間隔時間
   - 監控 API 回應標頭中的速率限制資訊

2. **S3 上傳失敗**:
   - 檢查 IAM 權限
   - 確認 S3 bucket 存在且可寫入
   - 檢查網路連線狀態

3. **記憶體不足**:
   - 調整 Lambda 記憶體配置
   - 優化資料處理邏輯
   - 考慮分批處理大量資料

4. **超時問題**:
   - 增加 Lambda 超時設定
   - 優化 API 請求邏輯
   - 考慮使用非同步處理

### 除錯技巧

```bash
# 啟用詳細日誌
export LOG_LEVEL=DEBUG

# 本地測試特定功能
go run main.go -source=arxiv -date=2024-01-01

# 檢查配置載入
go run main.go -config-only

# 測試 S3 連線
go run main.go -test-s3
```

## 擴展指南

### 新增資料來源

1. **更新配置**:
   在 `config/pipeline-config.yaml` 新增資料來源配置

2. **實作 API 客戶端**:
   ```go
   // 在新的套件中實作客戶端
   package newsource
   
   type Client struct {
       endpoint string
       apiKey   string
   }
   
   func (c *Client) FetchPapers(query string) ([]Paper, error) {
       // 實作 API 調用邏輯
   }
   ```

3. **註冊資料來源**:
   在 `main.go` 中註冊新的資料來源處理器

4. **新增測試**:
   為新的資料來源新增完整的單元測試和整合測試

### 效能優化

1. **並行處理**: 使用 goroutines 並行處理多個 API 請求
2. **連線池**: 重用 HTTP 連線減少建立成本
3. **快取**: 對重複請求實作本地快取
4. **批次處理**: 將多個小請求合併為大請求

## 安全考量

- **API 金鑰管理**: 使用 AWS Secrets Manager 存儲敏感資訊
- **網路安全**: 限制出站網路存取
- **資料加密**: 確保 S3 存儲加密
- **存取控制**: 使用最小權限原則配置 IAM 角色