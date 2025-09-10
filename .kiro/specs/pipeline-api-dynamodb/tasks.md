# 實作計劃

- [x] 1. 建立專案結構和基礎配置
  - 創建 Go 和 Python 服務的目錄結構
  - 設定各服務的 Makefile 和基礎配置檔案
  - 建立 YAML 配置檔案結構支援多資料來源
  - _需求: 5.1, 5.5, 6.1_

- [x] 2. 建立統一 Logger 機制
  - [x] 2.1 建立共享 Logger 套件
    - 將 data-collector 的 logger 套件移動到 go-services/shared/logger/
    - 更新 logger 套件的 import 路徑和模組結構
    - 確保 Logger 介面支援所有必要方法 (Info, InfoWithCount, InfoWithDuration, Warn, Error, Debug)
    - 撰寫 shared logger 的單元測試 (同目錄 *_test.go)
    - 執行測試確保 logger 功能正確性
    - _需求: 4.6, 4.7_

  - [x] 2.2 更新 data-collector 使用 shared logger
    - 更新 data-collector 的 import 路徑使用 shared/logger
    - 驗證 data-collector 的所有日誌功能正常運作
    - 執行 data-collector 的完整測試套件
    - 確保日誌輸出格式和內容保持一致
    - _需求: 4.6, 4.8_

  - [x] 2.3 遷移 batch-processor 到統一 logger
    - 更新 batch-processor 使用 shared/logger 而非 monitoring/logger
    - 替換所有 log.Printf 調用為結構化 logger 方法
    - 更新錯誤處理使用 logger.Error() 方法
    - 移除或重構 monitoring/logger.go 避免重複功能
    - 撰寫遷移後的單元測試 (同目錄 *_test.go)
    - 執行完整測試套件確保功能正常
    - _需求: 4.6, 4.8, 4.9_

- [ ] 3. 實作資料收集服務 (Go)
  - [x] 3.1 建立 arXiv API 客戶端和資料結構
    - 實作 HTTP 客戶端連接 arXiv API
    - 定義論文資料結構和 JSON/XML 解析
    - 實作 API 回應格式適配邏輯
    - 撰寫 API 客戶端和資料結構的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 1.1_

  - [x] 3.2 實作配置管理和 S3 上傳功能
    - 建立 YAML 配置檔案讀取功能
    - 實作資料壓縮 (gzip) 和 S3 上傳
    - 加入日期戳記檔案命名邏輯
    - 撰寫配置管理和 S3 上傳的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 1.1, 1.2, 1.3_

  - [x] 3.3 加入結構化日誌和基本錯誤處理
    - 實作 CloudWatch 結構化日誌輸出
    - 加入基本錯誤處理和日誌記錄 (不含重試機制，由 Step Function 處理)
    - 撰寫日誌和錯誤處理的單元測試 (同目錄 *_test.go)
    - 執行完整測試套件確保所有功能正常
    - _需求: 4.1, 4.5_

- [x] 4. 實作批次處理服務 (Go)
  - [x] 4.1 建立 S3 事件處理和資料解壓縮
    - 實作 Lambda S3 事件觸發處理
    - 建立 S3 檔案下載和 gzip 解壓縮功能
    - 實作批次資料讀取和解析
    - 撰寫 S3 事件處理和解壓縮的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 2.1_

  - [x] 4.2 實作資料去重和 DynamoDB 操作
    - 建立資料去重邏輯，基於 paper_id 識別重複
    - 實作 DynamoDB 批次 upsert 操作 (每批 25 筆)
    - 加入 traceID 生成和批次時間戳記
    - 撰寫去重邏輯和 DynamoDB 操作的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 2.2, 2.3, 2.4_

  - [x] 4.3 實作 Step Function 回應和監控
    - 實作 ProcessResult 結構化回應給 Step Function
    - 加入處理進度和統計資訊日誌
    - 建立錯誤狀態回報機制 (讓 Step Function 處理重試)
    - 撰寫回應格式和監控的單元測試 (同目錄 *_test.go)
    - 執行完整測試套件確保所有功能正常
    - _需求: 2.5, 4.2, 4.3_

  - [x] 4.4 重構 batch-processor 移除 adapter 模式
    - 修改 processor 介面直接使用具體的統計類型而非 interface{}
    - 移除 adapters 目錄和相關 adapter 實現
    - 更新 processor.go 直接使用 deduplicator 和 dynamodb writer 的具體類型
    - 更新 main.go 移除 adapter 的創建和使用
    - 撰寫重構後的單元測試確保功能不變
    - 執行完整測試套件驗證重構正確性
    - _需求: 5.7, 5.8_

- [x] 5. 實作向量化 API 服務 (Python)
  - [x] 5.1 建立 Hugging Face 模型和 API 端點
    - 設定 Hugging Face transformers 模型載入
    - 實作 Flask/FastAPI 端點接受文字輸入
    - 建立向量生成和 JSON 回應格式
    - 撰寫模型載入和 API 端點的單元測試 (同目錄 test_*.py)
    - 執行測試確保功能正確性
    - _需求: 3.1, 3.3_

  - [x] 5.2 加入模型管理和錯誤處理
    - 實作模型預載入和記憶體管理
    - 加入 API 請求驗證和錯誤回應
    - 建立結構化日誌輸出
    - 撰寫模型管理和錯誤處理的單元測試 (同目錄 test_*.py)
    - 執行完整測試套件確保所有功能正常
    - _需求: 4.1, 4.5_

- [x] 6. 實作向量化協調服務 (Go)
  - [x] 6.1 建立 traceID 查詢和資料獲取
    - 實作根據 traceID 從 DynamoDB 查詢論文資料
    - 建立 GSI 查詢邏輯和分頁處理
    - 實作標題和摘要文字組合邏輯
    - 撰寫 traceID 查詢和資料獲取的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 3.2_

  - [x] 6.2 實作向量化 API 客戶端和結果存儲
    - 建立 HTTP 客戶端調用 Python 向量化 API
    - 實作向量結果解析和驗證
    - 建立 Vectors Table 的批次寫入功能
    - 撰寫 API 客戶端和結果存儲的單元測試 (同目錄 *_test.go)
    - 執行測試確保功能正確性
    - _需求: 3.1, 3.2_

  - [x] 6.3 加入錯誤處理和監控
    - 實作基本錯誤處理和狀態回報 (重試由 Step Function 處理)
    - 加入向量化處理進度追蹤
    - 建立結構化日誌和指標輸出
    - 撰寫錯誤處理和監控的單元測試 (同目錄 *_test.go)
    - 執行完整測試套件確保所有功能正常
    - _需求: 4.1, 4.2, 4.6_

- [x] 7. 建立 DynamoDB 資料表和索引
  - 建立 Papers Table 和 Vectors Table 的 CloudFormation 模板
  - 設定必要的 GSI (source+published_date, trace_id+batch_timestamp)
  - 配置資料表的讀寫容量和自動擴展
  - _需求: 2.3, 3.2_

- [x] 8. 實作部署和測試腳本
  - [x] 8.1 建立編譯和打包腳本
    - 為各 Go 服務建立交叉編譯 Makefile
    - 為 Python 服務建立依賴打包腳本
    - 建立 Lambda 部署 ZIP 檔案生成
    - 撰寫編譯和打包腳本的測試 (驗證產出檔案)
    - 執行測試確保腳本正確性
    - _需求: 6.1_

  - [x] 8.2 建立 AWS CLI 部署腳本
    - 實作 Lambda 函數建立和更新腳本
    - 建立 S3 bucket 和事件觸發配置
    - 加入部署驗證和健康檢查
    - 撰寫部署腳本的測試 (模擬部署流程)
    - 執行測試確保部署流程正確
    - _需求: 6.1_

- [x] 9. 建立整合測試和文件
  - [x] 9.1 撰寫專案文件和流程圖
    - 建立 README 包含高層次和模組流程圖
    - 撰寫各服務的 API 文件和使用說明
    - 建立部署和維護指南
    - 驗證文件完整性和準確性
    - _需求: 5.3_