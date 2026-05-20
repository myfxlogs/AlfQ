"""Tests for ALFQ model trainer."""
import pytest
from alfq_research.model.trainer import ModelTrainer


def test_trainer_default():
    trainer = ModelTrainer()
    assert trainer.model_type == "lightgbm"


def test_trainer_custom():
    trainer = ModelTrainer(model_type="sklearn")
    assert trainer.model_type == "sklearn"


@pytest.mark.skip(reason="requires lightgbm and numpy")
def test_trainer_lightgbm_basic():
    import numpy as np
    trainer = ModelTrainer(model_type="lightgbm")
    X = np.random.rand(100, 5)
    y = np.random.rand(100)
    model = trainer.train(X, y)
    assert model is not None


def test_trainer_unknown_type():
    trainer = ModelTrainer(model_type="unknown")
    with pytest.raises(ValueError, match="unknown model_type"):
        trainer.train([], [])
