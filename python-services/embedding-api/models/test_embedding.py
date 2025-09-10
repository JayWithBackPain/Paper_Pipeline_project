import pytest
import time
from unittest.mock import patch, MagicMock
import torch

from models.embedding import (
    ModelManager, 
    ModelLoadError, 
    ValidationError, 
    EmbeddingError
)


class TestModelManager:
    """Test cases for ModelManager class"""
    
    def setup_method(self):
        """Set up test fixtures"""
        self.model_manager = ModelManager("test-model")
    
    @patch('models.embedding.AutoTokenizer')
    @patch('models.embedding.AutoModel')
    def test_load_model_success(self, mock_model_class, mock_tokenizer_class):
        """Test successful model loading"""
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer
        mock_model_class.from_pretrained.return_value = mock_model
        
        model, tokenizer = self.model_manager.load_model()
        
        assert model == mock_model
        assert tokenizer == mock_tokenizer
        mock_model.eval.assert_called_once()
        assert self.model_manager._model_loaded_at is not None
    
    @patch('models.embedding.AutoTokenizer')
    @patch('models.embedding.AutoModel')
    def test_load_model_reuse(self, mock_model_class, mock_tokenizer_class):
        """Test that model is reused when already loaded"""
        # Pre-load model
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer
        mock_model_class.from_pretrained.return_value = mock_model
        
        # First load
        self.model_manager.load_model()
        
        # Reset mocks
        mock_tokenizer_class.reset_mock()
        mock_model_class.reset_mock()
        
        # Second load should reuse
        model, tokenizer = self.model_manager.load_model()
        
        mock_tokenizer_class.from_pretrained.assert_not_called()
        mock_model_class.from_pretrained.assert_not_called()
    
    @patch('models.embedding.AutoTokenizer')
    def test_load_model_failure(self, mock_tokenizer_class):
        """Test model loading failure"""
        mock_tokenizer_class.from_pretrained.side_effect = Exception("Download failed")
        
        with pytest.raises(ModelLoadError, match="Model loading failed"):
            self.model_manager.load_model()
    
    def test_should_reload_model_timeout(self):
        """Test model reload due to timeout"""
        # Set a very short timeout
        self.model_manager._model_timeout_seconds = 1
        self.model_manager._model_loaded_at = time.time() - 2  # 2 seconds ago
        
        assert self.model_manager._should_reload_model() is True
    
    def test_should_reload_model_no_timeout(self):
        """Test model not reloaded when within timeout"""
        self.model_manager._model_timeout_seconds = 3600
        self.model_manager._model_loaded_at = time.time()  # Just loaded
        
        assert self.model_manager._should_reload_model() is False
    
    def test_cleanup_model(self):
        """Test model cleanup"""
        # Set up mock model and tokenizer
        self.model_manager._model = MagicMock()
        self.model_manager._tokenizer = MagicMock()
        self.model_manager._model_loaded_at = time.time()
        
        self.model_manager._cleanup_model()
        
        assert self.model_manager._model is None
        assert self.model_manager._tokenizer is None
        assert self.model_manager._model_loaded_at is None
    
    @patch('models.embedding.ModelManager.load_model')
    def test_generate_embedding_success(self, mock_load_model):
        """Test successful embedding generation"""
        # Mock tokenizer and model
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_load_model.return_value = (mock_model, mock_tokenizer)
        
        # Mock tokenizer output
        mock_inputs = {'input_ids': torch.tensor([[1, 2, 3]]), 'attention_mask': torch.tensor([[1, 1, 1]])}
        mock_tokenizer.return_value = mock_inputs
        
        # Mock model output
        mock_outputs = MagicMock()
        mock_outputs.last_hidden_state = torch.tensor([[[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]])
        mock_model.return_value = mock_outputs
        
        result = self.model_manager.generate_embedding("test text")
        
        assert isinstance(result, list)
        assert len(result) == 2  # Should match the embedding dimension
        assert all(isinstance(x, float) for x in result)
    
    def test_generate_embedding_empty_text(self):
        """Test embedding generation with empty text"""
        with pytest.raises(ValidationError, match="Input text cannot be empty"):
            self.model_manager.generate_embedding("")
    
    def test_generate_embedding_whitespace_only(self):
        """Test embedding generation with whitespace-only text"""
        with pytest.raises(ValidationError, match="Input text cannot be empty"):
            self.model_manager.generate_embedding("   ")
    
    @patch('models.embedding.ModelManager.load_model')
    def test_generate_embedding_long_text(self, mock_load_model):
        """Test embedding generation with very long text"""
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_load_model.return_value = (mock_model, mock_tokenizer)
        
        # Mock successful processing
        mock_inputs = {'input_ids': torch.tensor([[1, 2, 3]])}
        mock_tokenizer.return_value = mock_inputs
        
        mock_outputs = MagicMock()
        mock_outputs.last_hidden_state = torch.tensor([[[0.1, 0.2]]])
        mock_model.return_value = mock_outputs
        
        # Create very long text
        long_text = "This is a very long text " * 1000
        
        result = self.model_manager.generate_embedding(long_text)
        
        # Should still work but text should be truncated
        assert isinstance(result, list)
        # Verify the text was truncated to max_length
        call_args = mock_tokenizer.call_args[0]
        assert len(call_args[0]) <= 8192  # max_length limit
    
    @patch('models.embedding.ModelManager.load_model')
    def test_generate_embedding_tokenization_error(self, mock_load_model):
        """Test embedding generation when tokenization fails"""
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_load_model.return_value = (mock_model, mock_tokenizer)
        
        mock_tokenizer.side_effect = Exception("Tokenization failed")
        
        with pytest.raises(ValidationError, match="Text tokenization failed"):
            self.model_manager.generate_embedding("test text")
    
    @patch('models.embedding.ModelManager.load_model')
    def test_generate_embedding_model_inference_error(self, mock_load_model):
        """Test embedding generation when model inference fails"""
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_load_model.return_value = (mock_model, mock_tokenizer)
        
        mock_tokenizer.return_value = {'input_ids': torch.tensor([[1, 2, 3]])}
        mock_model.side_effect = Exception("Model inference failed")
        
        with pytest.raises(EmbeddingError, match="Embedding generation failed"):
            self.model_manager.generate_embedding("test text")
    
    @patch('models.embedding.ModelManager.load_model')
    def test_generate_embedding_cuda_oom(self, mock_load_model):
        """Test embedding generation with CUDA out of memory error"""
        mock_tokenizer = MagicMock()
        mock_model = MagicMock()
        mock_load_model.return_value = (mock_model, mock_tokenizer)
        
        mock_tokenizer.return_value = {'input_ids': torch.tensor([[1, 2, 3]])}
        mock_model.side_effect = torch.cuda.OutOfMemoryError("CUDA out of memory")
        
        with pytest.raises(EmbeddingError, match="GPU memory exhausted"):
            self.model_manager.generate_embedding("test text")
    
    def test_get_model_info(self):
        """Test getting model information"""
        info = self.model_manager.get_model_info()
        
        assert isinstance(info, dict)
        assert 'model_name' in info
        assert 'model_loaded' in info
        assert 'tokenizer_loaded' in info
        assert info['model_name'] == "test-model"
        assert info['model_loaded'] is False  # Not loaded yet
        assert info['tokenizer_loaded'] is False  # Not loaded yet


class TestExceptions:
    """Test custom exception classes"""
    
    def test_model_load_error(self):
        """Test ModelLoadError exception"""
        with pytest.raises(ModelLoadError):
            raise ModelLoadError("Test error")
    
    def test_validation_error(self):
        """Test ValidationError exception"""
        with pytest.raises(ValidationError):
            raise ValidationError("Test validation error")
    
    def test_embedding_error(self):
        """Test EmbeddingError exception"""
        with pytest.raises(EmbeddingError):
            raise EmbeddingError("Test embedding error")