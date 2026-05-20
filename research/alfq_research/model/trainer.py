"""ALFQ Model Trainer — LightGBM/sklearn wrapper with ONNX export.

Usage:
    from alfq_research.model import ModelTrainer
    trainer = ModelTrainer(model_type="lightgbm")
    model = trainer.train(X, y)
    path = trainer.export_onnx(model, "model.onnx", ["f1", "f2"])
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass
class ModelTrainer:
    """Lightweight ML training wrapper.

    Parameters
    ----------
    model_type:
        "lightgbm" | "random_forest" | "linear"
    params:
        Model hyperparameters dict passed to the underlying constructor.
    """

    model_type: str = "lightgbm"
    params: dict[str, Any] | None = None

    _MODEL_MAP = {
        "lightgbm": ("lightgbm", "LGBMRegressor"),
        "random_forest": ("sklearn.ensemble", "RandomForestRegressor"),
        "linear": ("sklearn.linear_model", "LinearRegression"),
    }

    def train(self, X: Any, y: Any) -> Any:
        """Train a model and return it.

        Parameters
        ----------
        X:
            Feature matrix (numpy array, pandas DataFrame, or list).
        y:
            Target vector.

        Returns
        -------
        Trained model object.
        """
        import numpy as np

        if not isinstance(X, np.ndarray):
            X = np.asarray(X, dtype=np.float32)
        if not isinstance(y, np.ndarray):
            y = np.asarray(y, dtype=np.float32)

        if self.model_type not in self._MODEL_MAP:
            raise ValueError(f"unknown model_type: {self.model_type}")

        module_name, class_name = self._MODEL_MAP[self.model_type]
        mod = __import__(module_name, fromlist=[class_name])
        cls = getattr(mod, class_name)

        kwargs = self.params or {}
        if self.model_type == "lightgbm":
            kwargs.setdefault("n_estimators", 100)
            kwargs.setdefault("verbosity", -1)
        elif self.model_type == "random_forest":
            kwargs.setdefault("n_estimators", 100)

        model = cls(**kwargs)
        model.fit(X, y)
        return model

    def export_onnx(
        self,
        model: Any,
        output_path: str | Path,
        input_features: list[str],
    ) -> str:
        """Export trained model to ONNX format."""
        from .exporter import export_onnx
        return export_onnx(model, output_path, input_features)


# ── Standalone convenience ──

def train_lightgbm(
    X: Any,
    y: Any,
    n_estimators: int = 100,
    **kwargs: Any,
) -> Any:
    """Quick one-liner: train a LightGBM regressor."""
    import lightgbm as lgb
    model = lgb.LGBMRegressor(n_estimators=n_estimators, verbosity=-1, **kwargs)
    model.fit(X, y)
    return model
