"""
Model management for embedding generation
"""
import logging
import os
import time
from typing import List, Optional, Tuple
from transformers import AutoTokenizer, AutoModel
import torch
import gc

logger = logging.getLogger(__name__)


class ModelManager:
    """Manages Hugging Face model loading and memory management"""
    
    def __init__(self, model_name: str = "sentence-transformers/all-MiniLM-L6-v2"):
        self.model_name = model_name
        self._model: Optional[AutoModel] = None
        self._tokenizer: Optional[AutoTokenizer] = None
        self._model_loaded_at: Optional[float] = None
        self._max_memory_mb = int(os.getenv('MAX_MEMORY_MB', '512'))
        self._model_timeout_seconds = int(os.getenv('MODEL_TIMEOUT_SECONDS', '3600'))  # 1 hour
    
    def load_model(self) -> Tuple[AutoModel, AutoTokenizer]:
        """Load the Hugging Face model and tokenizer with memory management"""
        try:
            # Check if model needs to be reloaded due to timeout
            if self._should_reload_model():
                self._cleanup_model()
            
            if self._model is None or self._tokenizer is None:
                logger.info(f"Loading model: {self.model_name}")
                start_time = time.time()
                
                # Load tokenizer first (smaller memory footprint)
                self._tokenizer = AutoTokenizer.from_pretrained(self.model_name)
                
                # Load model with memory optimization
                self._model = AutoModel.from_pretrained(
                    self.model_name,
                    torch_dtype=torch.float32,  # Use float32 for better compatibility
                    device_map="auto" if torch.cuda.is_available() else None
                )
                self._model.eval()  # Set to evaluation mode
                
                # Record load time
                load_time = time.time() - start_time
                self._model_loaded_at = time.time()
                
                logger.info(f"Model loaded successfully in {load_time:.2f} seconds")
                
                # Log memory usage if available
                self._log_memory_usage()
            
            return self._model, self._tokenizer
            
        except Exception as e:
            logger.error(f"Failed to load model {self.model_name}: {str(e)}")
            self._cleanup_model()
            raise ModelLoadError(f"Model loading failed: {str(e)}")
    
    def _should_reload_model(self) -> bool:
        """Check if model should be reloaded due to timeout"""
        if self._model_loaded_at is None:
            return False
        
        time_since_load = time.time() - self._model_loaded_at
        return time_since_load > self._model_timeout_seconds
    
    def _cleanup_model(self):
        """Clean up model from memory"""
        if self._model is not None:
            logger.info("Cleaning up model from memory")
            del self._model
            self._model = None
        
        if self._tokenizer is not None:
            del self._tokenizer
            self._tokenizer = None
        
        self._model_loaded_at = None
        
        # Force garbage collection
        gc.collect()
        
        # Clear CUDA cache if available
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
    
    def _log_memory_usage(self):
        """Log current memory usage"""
        try:
            import psutil
            process = psutil.Process(os.getpid())
            memory_mb = process.memory_info().rss / 1024 / 1024
            logger.info(f"Current memory usage: {memory_mb:.1f} MB")
            
            if memory_mb > self._max_memory_mb:
                logger.warning(f"Memory usage ({memory_mb:.1f} MB) exceeds limit ({self._max_memory_mb} MB)")
        except ImportError:
            # psutil not available, skip memory logging
            pass
    
    def generate_embedding(self, text: str) -> List[float]:
        """Generate embedding vector for input text with error handling"""
        if not text or not text.strip():
            raise ValidationError("Input text cannot be empty")
        
        # Validate text length
        max_length = 8192  # Reasonable limit for most models
        if len(text) > max_length:
            logger.warning(f"Text length ({len(text)}) exceeds maximum ({max_length}), truncating")
            text = text[:max_length]
        
        try:
            model, tokenizer = self.load_model()
            
            start_time = time.time()
            
            # Tokenize input text with proper error handling
            try:
                inputs = tokenizer(
                    text, 
                    return_tensors="pt", 
                    truncation=True, 
                    padding=True, 
                    max_length=512
                )
            except Exception as e:
                raise ValidationError(f"Text tokenization failed: {str(e)}")
            
            # Generate embeddings
            try:
                with torch.no_grad():
                    outputs = model(**inputs)
                    # Use mean pooling of last hidden states
                    embeddings = outputs.last_hidden_state.mean(dim=1)
                    # Normalize the embeddings
                    embeddings = torch.nn.functional.normalize(embeddings, p=2, dim=1)
                
                # Convert to list of floats
                embedding_list = embeddings.squeeze().numpy().tolist()
                
                # Validate embedding output
                if not embedding_list or not isinstance(embedding_list, list):
                    raise EmbeddingError("Generated embedding is invalid")
                
                processing_time = time.time() - start_time
                logger.info(f"Generated embedding with dimension {len(embedding_list)} in {processing_time:.3f}s")
                
                return embedding_list
                
            except torch.cuda.OutOfMemoryError:
                logger.error("CUDA out of memory during inference")
                self._cleanup_model()
                raise EmbeddingError("GPU memory exhausted during embedding generation")
            except Exception as e:
                logger.error(f"Model inference failed: {str(e)}")
                raise EmbeddingError(f"Embedding generation failed: {str(e)}")
                
        except (ModelLoadError, ValidationError, EmbeddingError):
            # Re-raise our custom exceptions
            raise
        except Exception as e:
            logger.error(f"Unexpected error during embedding generation: {str(e)}")
            raise EmbeddingError(f"Unexpected error: {str(e)}")
    
    def get_model_info(self) -> dict:
        """Get information about the current model"""
        return {
            "model_name": self.model_name,
            "model_loaded": self._model is not None,
            "tokenizer_loaded": self._tokenizer is not None,
            "loaded_at": self._model_loaded_at,
            "max_memory_mb": self._max_memory_mb,
            "timeout_seconds": self._model_timeout_seconds
        }


class ModelLoadError(Exception):
    """Exception raised when model loading fails"""
    pass


class ValidationError(Exception):
    """Exception raised when input validation fails"""
    pass


class EmbeddingError(Exception):
    """Exception raised when embedding generation fails"""
    pass