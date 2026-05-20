"""ONNX roundtrip test: train → export → load → predict consistency."""

import tempfile
from pathlib import Path

import numpy as np
import pytest

from alfq_research.model import (
    ModelTrainer,
    train_lightgbm,
    export_onnx,
    load_onnx,
    predict_onnx,
)


def make_regression_data(n_samples: int = 200, n_features: int = 5):
    """Generate synthetic regression data."""
    rng = np.random.default_rng(42)
    X = rng.normal(0, 1, (n_samples, n_features))
    true_coef = rng.normal(0, 1, n_features)
    y = X @ true_coef + rng.normal(0, 0.1, n_samples)
    return X, y


def test_train_lightgbm():
    X, y = make_regression_data()
    model = train_lightgbm(X, y, n_estimators=50)
    preds = model.predict(X[:5])
    assert len(preds) == 5
    assert preds.dtype == np.float64 or preds.dtype == np.float32


def test_model_trainer_lgb():
    trainer = ModelTrainer(model_type="lightgbm")
    X, y = make_regression_data()
    model = trainer.train(X, y)
    assert model is not None
    preds = model.predict(X[:10])
    assert len(preds) == 10


def test_model_trainer_rf():
    trainer = ModelTrainer(model_type="random_forest")
    X, y = make_regression_data(100)
    model = trainer.train(X, y)
    assert model is not None


def test_onnx_roundtrip():
    """Train LGBM → export ONNX → load → predict → compare."""
    X, y = make_regression_data(200)
    model = train_lightgbm(X, y, n_estimators=50)
    sk_preds = model.predict(X[:10])

    with tempfile.TemporaryDirectory() as tmpdir:
        onnx_path = Path(tmpdir) / "test_model.onnx"
        features = [f"f{i}" for i in range(X.shape[1])]

        exported = export_onnx(model, str(onnx_path), features)
        assert onnx_path.exists()
        assert exported == str(onnx_path.resolve())

        session = load_onnx(str(onnx_path))
        onnx_preds = predict_onnx(session, X[:10])

        # Predictions should be close (ONNX may have minor float differences)
        onnx_flat = onnx_preds.flatten()
        assert len(onnx_flat) == len(sk_preds)
        max_diff = np.max(np.abs(sk_preds - onnx_flat))
        assert max_diff < 1e-4, f"ONNX vs sklearn max diff: {max_diff}"


def test_onnx_multiple_outputs():
    """Verify output shape matches input samples."""
    X, y = make_regression_data(100, 3)
    model = train_lightgbm(X, y, n_estimators=20)

    with tempfile.TemporaryDirectory() as tmpdir:
        path = Path(tmpdir) / "m.onnx"
        export_onnx(model, str(path), ["a", "b", "c"])
        session = load_onnx(str(path))
        preds = predict_onnx(session, X)
        assert preds.shape[0] == 100


def test_model_trainer_export():
    trainer = ModelTrainer(model_type="lightgbm", params={"n_estimators": 30})
    X, y = make_regression_data(150)
    model = trainer.train(X, y)

    with tempfile.TemporaryDirectory() as tmpdir:
        path = trainer.export_onnx(model, f"{tmpdir}/m.onnx", ["f1", "f2", "f3", "f4", "f5"])
        assert Path(path).exists()


def test_trainer_unknown_type():
    trainer = ModelTrainer(model_type="xgboost")
    X, y = make_regression_data(10)
    with pytest.raises(ValueError, match="unknown model_type"):
        trainer.train(X, y)
