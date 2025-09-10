"""
API endpoints and request handling for embedding service
"""
import json
import logging
import time
from typing import Dict, Any, Optional
from models.embedding import ModelManager, ModelLoadError, ValidationError, EmbeddingError

logger = logging.getLogger(__name__)


class EmbeddingAPI:
    """Main API class for handling embedding requests"""
    
    def __init__(self, model_name: str = "sentence-transformers/all-MiniLM-L6-v2"):
        self.model_manager = ModelManager(model_name)
        self.request_count = 0
        self.total_processing_time = 0.0
    
    def process_request(self, event: Dict[str, Any], context: Any = None) -> Dict[str, Any]:
        """Process embedding request with comprehensive error handling"""
        request_id = self._get_request_id(context)
        start_time = time.time()
        
        try:
            # Log request start
            logger.info(f"Processing embedding request", extra={
                "request_id": request_id,
                "event_keys": list(event.keys()) if isinstance(event, dict) else "non-dict"
            })
            
            # Parse and validate request
            request_data = self._parse_request(event)
            text = self._validate_request(request_data)
            
            # Generate embedding
            embedding = self.model_manager.generate_embedding(text)
            
            # Prepare response
            response_data = {
                "embedding": embedding,
                "model_version": self.model_manager.model_name,
                "dimension": len(embedding),
                "processing_time_ms": int((time.time() - start_time) * 1000)
            }
            
            # Update statistics
            self.request_count += 1
            self.total_processing_time += time.time() - start_time
            
            # Log successful response
            logger.info(f"Successfully generated embedding", extra={
                "request_id": request_id,
                "text_length": len(text),
                "embedding_dimension": len(embedding),
                "processing_time_ms": response_data["processing_time_ms"]
            })
            
            return self._create_response(200, response_data)
            
        except ValidationError as e:
            logger.warning(f"Validation error: {str(e)}", extra={"request_id": request_id})
            return self._create_error_response(400, "VALIDATION_ERROR", str(e))
            
        except ModelLoadError as e:
            logger.error(f"Model loading error: {str(e)}", extra={"request_id": request_id})
            return self._create_error_response(503, "MODEL_LOAD_ERROR", "Model temporarily unavailable")
            
        except EmbeddingError as e:
            logger.error(f"Embedding generation error: {str(e)}", extra={"request_id": request_id})
            return self._create_error_response(500, "EMBEDDING_ERROR", "Failed to generate embedding")
            
        except Exception as e:
            logger.error(f"Unexpected error: {str(e)}", extra={
                "request_id": request_id,
                "error_type": type(e).__name__
            })
            return self._create_error_response(500, "INTERNAL_ERROR", "Internal server error")
    
    def _parse_request(self, event: Dict[str, Any]) -> Dict[str, Any]:
        """Parse request from Lambda event"""
        try:
            # Handle API Gateway format
            if 'body' in event:
                if isinstance(event['body'], str):
                    return json.loads(event['body'])
                else:
                    return event['body']
            # Handle direct invocation format
            else:
                return event
        except json.JSONDecodeError as e:
            raise ValidationError(f"Invalid JSON in request body: {str(e)}")
        except Exception as e:
            raise ValidationError(f"Failed to parse request: {str(e)}")
    
    def _validate_request(self, request_data: Dict[str, Any]) -> str:
        """Validate request data and extract text"""
        if not isinstance(request_data, dict):
            raise ValidationError("Request must be a JSON object")
        
        text = request_data.get('text', '')
        
        if not text:
            raise ValidationError("Text field is required and cannot be empty")
        
        if not isinstance(text, str):
            raise ValidationError("Text field must be a string")
        
        # Additional validation
        if len(text.strip()) == 0:
            raise ValidationError("Text cannot be only whitespace")
        
        return text.strip()
    
    def _get_request_id(self, context: Any) -> str:
        """Extract request ID from Lambda context"""
        if context and hasattr(context, 'aws_request_id'):
            return context.aws_request_id
        return f"req_{int(time.time() * 1000)}"
    
    def _create_response(self, status_code: int, data: Dict[str, Any]) -> Dict[str, Any]:
        """Create standardized API response"""
        return {
            'statusCode': status_code,
            'headers': {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*',  # Configure as needed
                'Access-Control-Allow-Methods': 'POST, OPTIONS',
                'Access-Control-Allow-Headers': 'Content-Type'
            },
            'body': json.dumps(data, ensure_ascii=False)
        }
    
    def _create_error_response(self, status_code: int, error_code: str, message: str) -> Dict[str, Any]:
        """Create standardized error response"""
        error_data = {
            "error": {
                "code": error_code,
                "message": message,
                "timestamp": int(time.time())
            }
        }
        return self._create_response(status_code, error_data)
    
    def get_health_status(self) -> Dict[str, Any]:
        """Get health status of the service"""
        model_info = self.model_manager.get_model_info()
        
        return {
            "status": "healthy" if model_info["model_loaded"] else "initializing",
            "model_info": model_info,
            "statistics": {
                "request_count": self.request_count,
                "average_processing_time_ms": (
                    int((self.total_processing_time / self.request_count) * 1000) 
                    if self.request_count > 0 else 0
                )
            },
            "timestamp": int(time.time())
        }
    
    def handle_options_request(self) -> Dict[str, Any]:
        """Handle CORS preflight requests"""
        return {
            'statusCode': 200,
            'headers': {
                'Access-Control-Allow-Origin': '*',
                'Access-Control-Allow-Methods': 'POST, OPTIONS',
                'Access-Control-Allow-Headers': 'Content-Type'
            },
            'body': ''
        }