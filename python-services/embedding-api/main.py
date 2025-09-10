import json
import os
import logging
from typing import Dict, Any
from api.endpoints import EmbeddingAPI

# Configure structured logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Global API instance for Lambda reuse
_api_instance = None


def get_api_instance() -> EmbeddingAPI:
    """Get or create API instance for Lambda reuse"""
    global _api_instance
    
    if _api_instance is None:
        model_name = os.getenv('MODEL_NAME', 'sentence-transformers/all-MiniLM-L6-v2')
        _api_instance = EmbeddingAPI(model_name)
        logger.info(f"Initialized embedding API with model: {model_name}")
    
    return _api_instance


def lambda_handler(event: Dict[str, Any], context: Any) -> Dict[str, Any]:
    """
    AWS Lambda handler for embedding API with comprehensive error handling
    """
    try:
        # Handle CORS preflight requests
        if event.get('httpMethod') == 'OPTIONS':
            api = get_api_instance()
            return api.handle_options_request()
        
        # Handle health check requests
        if event.get('path') == '/health' or event.get('httpMethod') == 'GET':
            api = get_api_instance()
            health_status = api.get_health_status()
            return {
                'statusCode': 200,
                'headers': {'Content-Type': 'application/json'},
                'body': json.dumps(health_status)
            }
        
        # Process embedding request
        api = get_api_instance()
        return api.process_request(event, context)
        
    except Exception as e:
        logger.error(f"Unhandled error in lambda_handler: {str(e)}", exc_info=True)
        return {
            'statusCode': 500,
            'headers': {'Content-Type': 'application/json'},
            'body': json.dumps({
                'error': {
                    'code': 'HANDLER_ERROR',
                    'message': 'Lambda handler error'
                }
            })
        }


def local_run():
    """Local development mode with enhanced testing"""
    print("Embedding API Service - Local Development Mode")
    
    # Test basic functionality
    test_event = {
        'text': 'This is a test paper title and abstract about machine learning and natural language processing'
    }
    
    result = lambda_handler(test_event, None)
    print(f"Basic test result: {result}")
    
    # Test API Gateway format
    api_gateway_event = {
        'body': json.dumps({'text': 'Another test with API Gateway format'})
    }
    
    result2 = lambda_handler(api_gateway_event, None)
    print(f"API Gateway test result: {result2}")
    
    # Test health check
    health_event = {'httpMethod': 'GET', 'path': '/health'}
    health_result = lambda_handler(health_event, None)
    print(f"Health check result: {health_result}")
    
    # Test error handling
    error_event = {'text': ''}
    error_result = lambda_handler(error_event, None)
    print(f"Error handling test: {error_result}")
    
    # Test CORS
    cors_event = {'httpMethod': 'OPTIONS'}
    cors_result = lambda_handler(cors_event, None)
    print(f"CORS test result: {cors_result}")


if __name__ == "__main__":
    if os.getenv('AWS_LAMBDA_FUNCTION_NAME'):
        # Running in Lambda
        pass
    else:
        # Running locally
        local_run()