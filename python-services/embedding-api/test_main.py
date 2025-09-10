import pytest
import json
from unittest.mock import patch, MagicMock

from main import lambda_handler, get_api_instance


class TestLambdaHandler:
    """Test cases for the main lambda handler"""
    
    @patch('main.get_api_instance')
    def test_lambda_handler_embedding_request(self, mock_get_api):
        """Test lambda handler with embedding request"""
        mock_api = MagicMock()
        mock_get_api.return_value = mock_api
        mock_api.process_request.return_value = {
            'statusCode': 200,
            'body': json.dumps({'embedding': [0.1, 0.2, 0.3]})
        }
        
        event = {'text': 'Test text'}
        result = lambda_handler(event, None)
        
        assert result['statusCode'] == 200
        mock_api.process_request.assert_called_once_with(event, None)
    
    @patch('main.get_api_instance')
    def test_lambda_handler_options_request(self, mock_get_api):
        """Test lambda handler with CORS preflight request"""
        mock_api = MagicMock()
        mock_get_api.return_value = mock_api
        mock_api.handle_options_request.return_value = {
            'statusCode': 200,
            'headers': {'Access-Control-Allow-Origin': '*'},
            'body': ''
        }
        
        event = {'httpMethod': 'OPTIONS'}
        result = lambda_handler(event, None)
        
        assert result['statusCode'] == 200
        mock_api.handle_options_request.assert_called_once()
    
    @patch('main.get_api_instance')
    def test_lambda_handler_health_check(self, mock_get_api):
        """Test lambda handler with health check request"""
        mock_api = MagicMock()
        mock_get_api.return_value = mock_api
        mock_api.get_health_status.return_value = {
            'status': 'healthy',
            'model_info': {'model_loaded': True}
        }
        
        event = {'httpMethod': 'GET', 'path': '/health'}
        result = lambda_handler(event, None)
        
        assert result['statusCode'] == 200
        assert 'Content-Type' in result['headers']
        mock_api.get_health_status.assert_called_once()
    
    @patch('main.get_api_instance')
    def test_lambda_handler_unhandled_exception(self, mock_get_api):
        """Test lambda handler with unhandled exception"""
        mock_get_api.side_effect = Exception("Initialization failed")
        
        event = {'text': 'Test text'}
        result = lambda_handler(event, None)
        
        assert result['statusCode'] == 500
        body = json.loads(result['body'])
        assert body['error']['code'] == 'HANDLER_ERROR'


class TestAPIInstance:
    """Test cases for API instance management"""
    
    @patch('main.EmbeddingAPI')
    def test_get_api_instance_creation(self, mock_api_class):
        """Test API instance creation"""
        # Reset global instance
        import main
        main._api_instance = None
        
        mock_api = MagicMock()
        mock_api_class.return_value = mock_api
        
        result = get_api_instance()
        
        assert result == mock_api
        mock_api_class.assert_called_once_with('sentence-transformers/all-MiniLM-L6-v2')
    
    @patch('main.EmbeddingAPI')
    def test_get_api_instance_reuse(self, mock_api_class):
        """Test API instance reuse"""
        # Set up existing instance
        import main
        existing_api = MagicMock()
        main._api_instance = existing_api
        
        result = get_api_instance()
        
        assert result == existing_api
        mock_api_class.assert_not_called()
    
    @patch.dict('os.environ', {'MODEL_NAME': 'custom-model'})
    @patch('main.EmbeddingAPI')
    def test_get_api_instance_custom_model(self, mock_api_class):
        """Test API instance creation with custom model"""
        # Reset global instance
        import main
        main._api_instance = None
        
        mock_api = MagicMock()
        mock_api_class.return_value = mock_api
        
        result = get_api_instance()
        
        mock_api_class.assert_called_once_with('custom-model')


class TestLocalRun:
    """Test cases for local development mode"""
    
    @patch('main.lambda_handler')
    def test_local_run_basic_functionality(self, mock_handler):
        """Test local run executes basic tests"""
        mock_handler.return_value = {'statusCode': 200, 'body': '{}'}
        
        # Import and run local_run function
        from main import local_run
        
        # This should not raise any exceptions
        local_run()
        
        # Verify lambda_handler was called multiple times for different test cases
        assert mock_handler.call_count >= 4  # Basic, API Gateway, health, error, CORS tests


class TestIntegration:
    """Integration tests for the complete main module"""
    
    @patch('main.get_api_instance')
    def test_complete_request_flow(self, mock_get_api):
        """Test complete request processing flow"""
        mock_api = MagicMock()
        mock_get_api.return_value = mock_api
        mock_api.process_request.return_value = {
            'statusCode': 200,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({
                'embedding': [0.1] * 384,
                'model_version': 'sentence-transformers/all-MiniLM-L6-v2',
                'dimension': 384,
                'processing_time_ms': 150
            })
        }
        
        # Simulate API Gateway event
        event = {
            'httpMethod': 'POST',
            'body': json.dumps({
                'text': 'Machine learning is transforming various industries.'
            })
        }
        
        mock_context = MagicMock()
        mock_context.aws_request_id = 'test-request-123'
        
        result = lambda_handler(event, mock_context)
        
        assert result['statusCode'] == 200
        body = json.loads(result['body'])
        assert len(body['embedding']) == 384
        assert body['model_version'] == 'sentence-transformers/all-MiniLM-L6-v2'
        assert 'processing_time_ms' in body
        
        # Verify the API was called correctly
        mock_api.process_request.assert_called_once_with(event, mock_context)