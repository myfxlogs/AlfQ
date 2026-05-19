"""ALFQ Model Trainer — LightGBM/sklearn wrapper with ONNX export."""
from __future__ import annotations
from dataclasses import dataclass
from typing import Any


@dataclass
class ModelTrainer:
    model_type: str = "lightgbm"

    def train(self, X: Any, y: Any) -> Any:
        if self.model_type == "lightgbm":
            try:
                import lightgbm as lgb
            except ImportError as err:
                raise ImportError("lightgbm not installed") from err
            model = lgb.LGBMRegressor(n_estimators=100)
            model.fit(X, y)
            return model
        return None

    def export_onnx(self, model: Any, feature_names: list[str], output_path: str):
        """Export model to ONNX format for strategy-svc deployment."""
        from skl2onnx import convert_sklearn
        from skl2onnx.common.data_types import FloatTensorType
        initial_type = [("float_input", FloatTensorType([None, len(feature_names)]))]
        onx = convert_sklearn(model, initial_types=initial_type)
        with open(output_path, "wb") as f:
            f.write(onx.SerializeToString())
