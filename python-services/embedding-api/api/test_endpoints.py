import pytest
import json
import time
from unittest.mock import patch, MagicMock

from api.endpoints import EmbeddingAPI
from models.embedding import ModelLoadError, ValidationError, EmbeddingError


class TestEmbeddingAPI:
    """Test cases for EmbeddingAPI class"""
    
    def setup_method(self):
        """Set up test fixtures"""
        self.api = EmbeddingAPI("test-model")
    
    @patch('api.endpoints.ModelManager')
    def test_process_request_success(self, mock_model_manager_class):
        """Test successful request processing"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.return_value = [0.1, 0.2, 0.3]
        mock_model_manager.model_name = "test-model"
        
        # Reinitialize API with mocked model manager
        self.api = EmbeddingAPI("test-model")
        
        event = {'text': 'Test text for embedding'}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 200
        body = json.loads(result['body'])
        assert 'embedding' in body
        assert 'model_version' in body
        assert 'dimension' in body
        assert 'processing_time_ms' in body
        assert body['embedding'] == [0.1, 0.2, 0.3]
        assert body['model_version'] == "test-model"
        assert body['dimension'] == 3
    
    @patch('api.endpoints.ModelManager')
    def test_process_request_api_gateway_format(self, mock_model_manager_class):
        """Test request processing with API Gateway format"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.return_value = [0.4, 0.5, 0.6]
        mock_model_manager.model_name = "test-model"
        
        self.api = EmbeddingAPI("test-model")
        
        event = {
            'body': json.dumps({'text': 'API Gateway test text'})
        }
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 200
        body = json.loads(result['body'])
        assert body['embedding'] == [0.4, 0.5, 0.6]
    
    def test_process_request_empty_text(self):
        """Test request processing with empty text"""
        event = {'text': ''}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 400
        body = json.loads(result['body'])
        assert 'error' in body
        assert body['error']['code'] == 'VALIDATION_ERROR'
    
    def test_process_request_missing_text(self):
        """Test request processing with missing text field"""
        event = {}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 400
        body = json.loads(result['body'])
        assert body['error']['code'] == 'VALIDATION_ERROR'
    
    def test_process_request_invalid_json(self):
        """Test request processing with invalid JSON"""
        event = {'body': 'invalid json'}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 400
        body = json.loads(result['body'])
        assert body['error']['code'] == 'VALIDATION_ERROR'
    
    @patch('api.endpoints.ModelManager')
    def test_process_request_model_load_error(self, mock_model_manager_class):
        """Test request processing when model loading fails"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.side_effect = ModelLoadError("Model failed to load")
        
        self.api = EmbeddingAPI("test-model")
        
        event = {'text': 'Test text'}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 503
        body = json.loads(result['body'])
        assert body['error']['code'] == 'MODEL_LOAD_ERROR'
    
    @patch('api.endpoints.ModelManager')
    def test_process_request_embedding_error(self, mock_model_manager_class):
        """Test request processing when embedding generation fails"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.side_effect = EmbeddingError("Embedding failed")
        
        self.api = EmbeddingAPI("test-model")
        
        event = {'text': 'Test text'}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 500
        body = json.loads(result['body'])
        assert body['error']['code'] == 'EMBEDDING_ERROR'
    
    @patch('api.endpoints.ModelManager')
    def test_process_request_unexpected_error(self, mock_model_manager_class):
        """Test request processing with unexpected error"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.side_effect = RuntimeError("Unexpected error")
        
        self.api = EmbeddingAPI("test-model")
        
        event = {'text': 'Test text'}
        result = self.api.process_request(event)
        
        assert result['statusCode'] == 500
        body = json.loads(result['body'])
        assert body['error']['code'] == 'INTERNAL_ERROR'
    
    def test_parse_request_direct_format(self):
        """Test parsing direct invocation format"""
        event = {'text': 'Direct test'}
        result = self.api._parse_request(event)
        
        assert result == {'text': 'Direct test'}
    
    def test_parse_request_api_gateway_string_body(self):
        """Test parsing API Gateway format with string body"""
        event = {'body': '{"text": "API Gateway test"}'}
        result = self.api._parse_request(event)
        
        assert result == {'text': 'API Gateway test'}
    
    def test_parse_request_api_gateway_dict_body(self):
        """Test parsing API Gateway format with dict body"""
        event = {'body': {'text': 'Dict body test'}}
        result = self.api._parse_request(event)
        
        assert result == {'text': 'Dict body test'}
    
    def test_validate_request_success(self):
        """Test successful request validation"""
        request_data = {'text': 'Valid text'}
        result = self.api._validate_request(request_data)
        
        assert result == 'Valid text'
    
    def test_validate_request_non_dict(self):
        """Test request validation with non-dict input"""
        with pytest.raises(ValidationError, match="Request must be a JSON object"):
            self.api._validate_request("not a dict")
    
    def test_validate_request_missing_text(self):
        """Test request validation with missing text field"""
        with pytest.raises(ValidationError, match="Text field is required"):
            self.api._validate_request({})
    
    def test_validate_request_empty_text(self):
        """Test request validation with empty text"""
        with pytest.raises(ValidationError, match="Text field is required"):
            self.api._validate_request({'text': ''})
    
    def test_validate_request_non_string_text(self):
        """Test request validation with non-string text"""
        with pytest.raises(ValidationError, match="Text field must be a string"):
            self.api._validate_request({'text': 123})
    
    def test_validate_request_whitespace_only(self):
        """Test request validation with whitespace-only text"""
        with pytest.raises(ValidationError, match="Text cannot be only whitespace"):
            self.api._validate_request({'text': '   '})
    
    def test_get_request_id_with_context(self):
        """Test request ID extraction with Lambda context"""
        mock_context = MagicMock()
        mock_context.aws_request_id = "test-request-id"
        
        result = self.api._get_request_id(mock_context)
        assert result == "test-request-id"
    
    def test_get_request_id_without_context(self):
        """Test request ID generation without context"""
        result = self.api._get_request_id(None)
        assert result.startswith("req_")
        assert len(result) > 4
    
    def test_create_response(self):
        """Test response creation"""
        data = {'test': 'data'}
        result = self.api._create_response(200, data)
        
        assert result['statusCode'] == 200
        assert 'headers' in result
        assert result['headers']['Content-Type'] == 'application/json'
        assert json.loads(result['body']) == data
    
    def test_create_error_response(self):
        """Test error response creation"""
        result = self.api._create_error_response(400, "TEST_ERROR", "Test message")
        
        assert result['statusCode'] == 400
        body = json.loads(result['body'])
        assert body['error']['code'] == "TEST_ERROR"
        assert body['error']['message'] == "Test message"
        assert 'timestamp' in body['error']
    
    @patch('api.endpoints.ModelManager')
    def test_get_health_status(self, mock_model_manager_class):
        """Test health status retrieval"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.get_model_info.return_value = {
            "model_loaded": True,
            "tokenizer_loaded": True
        }
        
        self.api = EmbeddingAPI("test-model")
        
        # Process a request to update statistics
        self.api.request_count = 5
        self.api.total_processing_time = 2.5
        
        result = self.api.get_health_status()
        
        assert result['status'] == 'healthy'
        assert 'model_info' in result
        assert 'statistics' in result
        assert result['statistics']['request_count'] == 5
        assert result['statistics']['average_processing_time_ms'] == 500  # 2.5s / 5 requests * 1000
    
    def test_handle_options_request(self):
        """Test CORS preflight request handling"""
        result = self.api.handle_options_request()
        
        assert result['statusCode'] == 200
        assert 'Access-Control-Allow-Origin' in result['headers']
        assert 'Access-Control-Allow-Methods' in result['headers']
        assert 'Access-Control-Allow-Headers' in result['headers']
        assert result['body'] == ''


class TestIntegration:
    """Integration tests for the complete API workflow"""
    
    @patch('api.endpoints.ModelManager')
    def test_complete_workflow(self, mock_model_manager_class):
        """Test complete request processing workflow"""
        mock_model_manager = MagicMock()
        mock_model_manager_class.return_value = mock_model_manager
        mock_model_manager.generate_embedding.return_value = [0.1] * 384
        mock_model_manager.model_name = "sentence-transformers/all-MiniLM-L6-v2"
        
        api = EmbeddingAPI()
        
        # Simulate API Gateway event
        event = {
            'httpMethod': 'POST',
            'body': json.dumps({
                'text': 'Machine learning is a subset of artificial intelligence.'
            })
        }
        
        mock_context = MagicMock()
        mock_context.aws_request_id = "test-request-123"
        
        result = api.process_request(event, mock_context)
        
        assert result['statusCode'] == 200
        body = json.loads(result['body'])
        assert len(body['embedding']) == 384
        assert body['model_version'] == "sentence-transformers/all-MiniLM-L6-v2"
        assert body['dimension'] == 384
        assert 'processing_time_ms' in body