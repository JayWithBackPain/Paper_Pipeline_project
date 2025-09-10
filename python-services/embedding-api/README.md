# Embedding API Service

向量化 API 服務，提供純粹的文字轉向量功能，使用 Hugging Face 預訓練模型。

## 功能概述

- 文字轉向量 API 服務
- 預載入 Hugging Face 模型
- 高效能向量生成
- 模型版本管理
- 健康檢查端點

## 技術規格

- **語言**: Python 3.11+
- **框架**: Flask 3.0+
- **觸發方式**: HTTP API (API Gateway)
- **記憶體**: 2048MB
- **超時**: 30秒
- **模型**: sentence-transformers/all-MiniLM-L6-v2

## 專案結構

```
embedding-api/
├── main.py                     # Lambda 入口點和 Flask 應用
├── test_main.py               # 主要邏輯測試
├── Makefile                   # 建置和部署腳本
├── requirements.txt           # Python 依賴
├── api/                       # API 端點
│   ├── __init__.py           # 套件初始化
│   ├── endpoints.py          # API 端點實作
│   └── test_endpoints.py     # 端點測試
├── models/                    # 模型管理
│   ├── __init__.py           # 套件初始化
│   ├── embedding.py          # 模型載入和推理
│   └── test_embedding.py     # 模型測試
├── package/                   # 部署套件目錄
└── venv/                      # 虛擬環境 (開發用)
```

## API 規格

### 端點說明

#### POST `/embed`
文字向量化端點

**請求格式**:
```json
{
  "text": "Deep learning approaches for natural language processing",
  "model_version": "v1.0",
  "normalize": true,
  "max_length": 512
}
```

**回應格式**:
```json
{
  "embedding": [0.1234, -0.5678, 0.9012, ...],
  "dimension": 384,
  "model_name": "all-MiniLM-L6-v2",
  "model_version": "v1.0",
  "processing_time_ms": 150,
  "text_length": 256,
  "normalized": true
}
```

**錯誤回應**:
```json
{
  "error": "Text too long",
  "error_code": "TEXT_TOO_LONG",
  "max_length": 512,
  "provided_length": 1024
}
```

#### GET `/health`
健康檢查端點

**回應格式**:
```json
{
  "status": "healthy",
  "model_loaded": true,
  "model_name": "all-MiniLM-L6-v2",
  "model_version": "v1.0",
  "memory_usage_mb": 1024,
  "uptime_seconds": 3600,
  "total_requests": 1500,
  "avg_response_time_ms": 145
}
```

#### GET `/info`
模型資訊端點

**回應格式**:
```json
{
  "model_info": {
    "name": "all-MiniLM-L6-v2",
    "version": "v1.0",
    "dimension": 384,
    "max_sequence_length": 256,
    "vocabulary_size": 30522,
    "model_size_mb": 90
  },
  "supported_languages": ["en", "zh", "es", "fr", "de"],
  "preprocessing": {
    "lowercase": true,
    "remove_special_chars": false,
    "truncate_strategy": "tail"
  }
}
```

## 核心功能

### 1. 模型管理器

**功能**:
- 模型預載入和快取
- 記憶體管理
- 模型版本控制
- 效能監控

**實作範例**:
```python
from sentence_transformers import SentenceTransformer
import torch
import time
import psutil
import logging

class EmbeddingModel:
    def __init__(self, model_name="all-MiniLM-L6-v2"):
        self.model_name = model_name
        self.model = None
        self.model_version = "v1.0"
        self.load_time = None
        self.request_count = 0
        self.total_inference_time = 0
        
        # 設定日誌
        logging.basicConfig(level=logging.INFO)
        self.logger = logging.getLogger(__name__)
        
    def load_model(self):
        """載入 Hugging Face 模型"""
        start_time = time.time()
        
        try:
            self.logger.info(f"Loading model: {self.model_name}")
            self.model = SentenceTransformer(self.model_name)
            
            # 設定為評估模式
            self.model.eval()
            
            # 如果有 GPU 可用，移到 GPU
            if torch.cuda.is_available():
                self.model = self.model.cuda()
                self.logger.info("Model loaded on GPU")
            else:
                self.logger.info("Model loaded on CPU")
                
            self.load_time = time.time() - start_time
            self.logger.info(f"Model loaded successfully in {self.load_time:.2f}s")
            
        except Exception as e:
            self.logger.error(f"Failed to load model: {str(e)}")
            raise
    
    def encode_text(self, text, normalize=True, max_length=512):
        """將文字編碼為向量"""
        if self.model is None:
            raise RuntimeError("Model not loaded")
        
        start_time = time.time()
        
        try:
            # 文字預處理
            processed_text = self._preprocess_text(text, max_length)
            
            # 生成向量
            with torch.no_grad():
                embedding = self.model.encode(
                    processed_text,
                    normalize_embeddings=normalize,
                    convert_to_tensor=False
                )
            
            # 更新統計
            inference_time = time.time() - start_time
            self.request_count += 1
            self.total_inference_time += inference_time
            
            return {
                "embedding": embedding.tolist(),
                "dimension": len(embedding),
                "processing_time_ms": int(inference_time * 1000),
                "text_length": len(processed_text),
                "normalized": normalize
            }
            
        except Exception as e:
            self.logger.error(f"Encoding failed: {str(e)}")
            raise
    
    def _preprocess_text(self, text, max_length):
        """文字預處理"""
        # 移除多餘空白
        text = " ".join(text.split())
        
        # 截斷過長文字
        if len(text) > max_length:
            text = text[:max_length]
            
        return text
    
    def get_model_info(self):
        """取得模型資訊"""
        if self.model is None:
            return None
            
        return {
            "name": self.model_name,
            "version": self.model_version,
            "dimension": self.model.get_sentence_embedding_dimension(),
            "max_sequence_length": self.model.max_seq_length,
            "model_size_mb": self._get_model_size_mb(),
            "device": str(self.model.device) if hasattr(self.model, 'device') else "cpu"
        }
    
    def get_health_status(self):
        """取得健康狀態"""
        memory_usage = psutil.Process().memory_info().rss / 1024 / 1024  # MB
        avg_response_time = (
            self.total_inference_time / self.request_count * 1000 
            if self.request_count > 0 else 0
        )
        
        return {
            "model_loaded": self.model is not None,
            "memory_usage_mb": int(memory_usage),
            "total_requests": self.request_count,
            "avg_response_time_ms": int(avg_response_time)
        }
    
    def _get_model_size_mb(self):
        """計算模型大小"""
        if self.model is None:
            return 0
            
        param_size = sum(p.numel() * p.element_size() for p in self.model.parameters())
        buffer_size = sum(b.numel() * b.element_size() for b in self.model.buffers())
        return (param_size + buffer_size) / 1024 / 1024
```

### 2. API 端點實作

**Flask 應用設定**:
```python
from flask import Flask, request, jsonify
import time
import logging

app = Flask(__name__)
model_manager = EmbeddingModel()

# 全域變數
start_time = time.time()
request_count = 0

@app.before_first_request
def load_model():
    """應用啟動時載入模型"""
    model_manager.load_model()

@app.before_request
def before_request():
    """請求前處理"""
    global request_count
    request_count += 1
    request.start_time = time.time()

@app.after_request
def after_request(response):
    """請求後處理"""
    processing_time = time.time() - request.start_time
    
    # 添加回應標頭
    response.headers['X-Processing-Time'] = f"{processing_time:.3f}"
    response.headers['X-Model-Version'] = model_manager.model_version
    
    return response

@app.route('/embed', methods=['POST'])
def embed_text():
    """文字向量化端點"""
    try:
        data = request.get_json()
        
        if not data or 'text' not in data:
            return jsonify({
                "error": "Missing required field: text",
                "error_code": "MISSING_TEXT"
            }), 400
        
        text = data['text']
        normalize = data.get('normalize', True)
        max_length = data.get('max_length', 512)
        
        # 驗證輸入
        if not isinstance(text, str) or len(text.strip()) == 0:
            return jsonify({
                "error": "Text must be a non-empty string",
                "error_code": "INVALID_TEXT"
            }), 400
        
        if len(text) > max_length:
            return jsonify({
                "error": "Text too long",
                "error_code": "TEXT_TOO_LONG",
                "max_length": max_length,
                "provided_length": len(text)
            }), 400
        
        # 生成向量
        result = model_manager.encode_text(text, normalize, max_length)
        
        # 添加模型資訊
        result.update({
            "model_name": model_manager.model_name,
            "model_version": model_manager.model_version
        })
        
        return jsonify(result)
        
    except Exception as e:
        logging.error(f"Embedding error: {str(e)}")
        return jsonify({
            "error": "Internal server error",
            "error_code": "INTERNAL_ERROR"
        }), 500

@app.route('/health', methods=['GET'])
def health_check():
    """健康檢查端點"""
    try:
        uptime = int(time.time() - start_time)
        health_status = model_manager.get_health_status()
        
        status = {
            "status": "healthy" if health_status["model_loaded"] else "unhealthy",
            "uptime_seconds": uptime,
            "total_requests": request_count,
            **health_status
        }
        
        return jsonify(status)
        
    except Exception as e:
        logging.error(f"Health check error: {str(e)}")
        return jsonify({
            "status": "unhealthy",
            "error": str(e)
        }), 500

@app.route('/info', methods=['GET'])
def model_info():
    """模型資訊端點"""
    try:
        model_info = model_manager.get_model_info()
        
        if model_info is None:
            return jsonify({
                "error": "Model not loaded",
                "error_code": "MODEL_NOT_LOADED"
            }), 503
        
        return jsonify({
            "model_info": model_info,
            "supported_languages": ["en", "zh", "es", "fr", "de"],
            "preprocessing": {
                "lowercase": False,
                "remove_special_chars": False,
                "truncate_strategy": "tail"
            }
        })
        
    except Exception as e:
        logging.error(f"Model info error: {str(e)}")
        return jsonify({
            "error": "Internal server error",
            "error_code": "INTERNAL_ERROR"
        }), 500

# Lambda 處理器
def lambda_handler(event, context):
    """AWS Lambda 處理器"""
    from werkzeug.serving import WSGIRequestHandler
    from io import StringIO
    import sys
    
    # 重定向輸出以捕獲 Flask 日誌
    old_stdout = sys.stdout
    sys.stdout = StringIO()
    
    try:
        # 處理 API Gateway 事件
        if 'httpMethod' in event:
            return handle_api_gateway_event(event, context)
        else:
            return {
                'statusCode': 400,
                'body': json.dumps({'error': 'Invalid event format'})
            }
    finally:
        sys.stdout = old_stdout

def handle_api_gateway_event(event, context):
    """處理 API Gateway 事件"""
    import json
    from werkzeug.test import Client
    from werkzeug.wrappers import Response
    
    # 建立測試客戶端
    client = Client(app, Response)
    
    # 轉換 API Gateway 事件為 WSGI 請求
    method = event['httpMethod']
    path = event['path']
    headers = event.get('headers', {})
    body = event.get('body', '')
    
    # 發送請求到 Flask 應用
    response = client.open(
        path=path,
        method=method,
        headers=headers,
        data=body
    )
    
    # 轉換回應為 API Gateway 格式
    return {
        'statusCode': response.status_code,
        'headers': dict(response.headers),
        'body': response.get_data(as_text=True)
    }
```

## 開發指南

### 本地開發

1. **建立虛擬環境** (需要 Python 3.11+):
   ```bash
   python3.11 -m venv venv
   source venv/bin/activate  # Linux/Mac
   # 或
   venv\Scripts\activate     # Windows
   ```

2. **安裝依賴**:
   ```bash
   pip install -r requirements.txt
   ```

3. **執行測試**:
   ```bash
   make test
   ```

4. **本地執行**:
   ```bash
   make local-run
   # 或
   python main.py
   ```

5. **測試 API**:
   ```bash
   # 測試向量化
   curl -X POST http://localhost:5000/embed \
     -H "Content-Type: application/json" \
     -d '{"text": "Hello world"}'
   
   # 測試健康檢查
   curl http://localhost:5000/health
   ```

### 建置和部署

1. **安裝依賴到套件目錄**:
   ```bash
   make install
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
# 執行所有測試
python -m pytest

# 執行特定測試
python -m pytest test_main.py -v
python -m pytest api/test_endpoints.py -v
python -m pytest models/test_embedding.py -v

# 執行測試並顯示覆蓋率
python -m pytest --cov=. --cov-report=html
```

#### 整合測試
```bash
# 啟動本地服務
python main.py &

# 執行整合測試
python -m pytest tests/integration/ -v

# 停止服務
pkill -f "python main.py"
```

#### 效能測試
```bash
# 使用 Apache Bench 測試
ab -n 1000 -c 10 -T application/json -p test_data.json http://localhost:5000/embed

# 使用 wrk 測試
wrk -t12 -c400 -d30s -s post.lua http://localhost:5000/embed
```

## 監控和日誌

### 結構化日誌範例

```json
{
  "timestamp": "2024-01-01T12:30:00Z",
  "level": "INFO",
  "message": "Embedding request processed",
  "service": "embedding-api",
  "request_id": "req-456def",
  "metadata": {
    "text_length": 256,
    "processing_time_ms": 150,
    "model_version": "v1.0",
    "normalized": true,
    "dimension": 384
  }
}
```

### 關鍵指標

- **回應時間**: API 平均回應時間
- **吞吐量**: 每秒處理的請求數
- **錯誤率**: 失敗請求 / 總請求
- **記憶體使用**: 模型記憶體使用量
- **模型載入時間**: 冷啟動時的模型載入時間

### CloudWatch 查詢

```bash
# 查看 API 效能
aws logs insights start-query \
  --log-group-name /aws/lambda/embedding-api \
  --start-time $(date -d '1 hour ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, @duration, metadata.processing_time_ms | filter message = "Embedding request processed" | stats avg(@duration), avg(metadata.processing_time_ms) by bin(5m)'

# 查看錯誤日誌
aws logs filter-log-events \
  --log-group-name /aws/lambda/embedding-api \
  --filter-pattern "ERROR"
```

## 故障排除

### 常見問題

1. **模型載入失敗**:
   ```bash
   # 檢查記憶體配置
   aws lambda get-function-configuration --function-name embedding-api
   
   # 增加記憶體
   aws lambda update-function-configuration \
     --function-name embedding-api \
     --memory-size 3008
   ```

2. **API 超時**:
   ```bash
   # 調整超時設定
   aws lambda update-function-configuration \
     --function-name embedding-api \
     --timeout 60
   ```

3. **記憶體不足**:
   ```python
   # 優化模型載入
   import torch
   torch.set_num_threads(1)  # 減少 CPU 執行緒
   ```

4. **冷啟動問題**:
   ```bash
   # 設定預配置並發
   aws lambda put-provisioned-concurrency-config \
     --function-name embedding-api \
     --provisioned-concurrency-config AllocatedProvisionedConcurrencyUnits=2
   ```

### 效能調優

**記憶體配置建議**:
- 小模型 (<100MB): 1024MB
- 中模型 (100-500MB): 2048MB
- 大模型 (>500MB): 3008MB

**CPU 優化**:
```python
import torch
import os

# 設定 CPU 執行緒數
torch.set_num_threads(int(os.environ.get('OMP_NUM_THREADS', '1')))

# 禁用梯度計算
torch.set_grad_enabled(False)
```

## 擴展指南

### 支援多模型

1. **模型註冊**:
   ```python
   class ModelRegistry:
       def __init__(self):
           self.models = {}
       
       def register_model(self, name, model_class):
           self.models[name] = model_class
       
       def get_model(self, name):
           return self.models.get(name)
   ```

2. **動態模型載入**:
   ```python
   @app.route('/embed/<model_name>', methods=['POST'])
   def embed_with_model(model_name):
       model = model_registry.get_model(model_name)
       if not model:
           return jsonify({"error": "Model not found"}), 404
       # 處理請求
   ```

### 效能優化

1. **批次處理**: 支援批次向量化
2. **模型量化**: 使用 ONNX 或 TensorRT 優化
3. **快取**: 實作向量快取機制
4. **非同步**: 使用 asyncio 提升並發

### 監控增強

1. **自定義指標**: 發送業務指標到 CloudWatch
2. **分散式追蹤**: 整合 AWS X-Ray
3. **告警設定**: 基於延遲和錯誤率的告警
4. **A/B 測試**: 支援多模型版本測試