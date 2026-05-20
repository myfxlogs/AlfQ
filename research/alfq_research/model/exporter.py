"""ALFQ ONNX exporter — sklearn/LightGBM → ONNX → MinIO upload.

Usage:
    from alfq_research.model import export_onnx, upload_model
    path = export_onnx(model, "momentum_v1.onnx", ["mom_20_60", "atr_14"])
    uri = upload_model("strategies/momentum/v1.onnx")
"""

from __future__ import annotations

from pathlib import Path
from typing import Any


def export_onnx(
    model: Any,
    output_path: str | Path,
    input_features: list[str],
    *,
    opset: int = 15,
) -> str:
    """Export a trained sklearn/LightGBM model to ONNX.

    Parameters
    ----------
    model:
        Trained sklearn-compatible model (LGBMRegressor, RandomForest, etc.).
    output_path:
        Output file path for the ONNX model.
    input_features:
        Ordered list of input feature names.
    opset:
        ONNX opset version.

    Returns
    -------
    str
        Absolute path to the exported ONNX file.
    """
    output_path = Path(output_path)
    n_features = len(input_features)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Try onnxmltools first (better LightGBM support), fall back to skl2onnx
    onx = _convert_onnxmltools(model, n_features, opset)
    if onx is None:
        onx = _convert_skl2onnx(model, output_path.stem, n_features, opset)

    output_path.write_bytes(onx.SerializeToString())
    return str(output_path.resolve())


def _convert_onnxmltools(model: Any, n_features: int, opset: int) -> Any | None:
    try:
        from onnxmltools import convert_lightgbm
        from onnxmltools.convert.common.data_types import FloatTensorType
    except ImportError:
        return None

    try:
        initial_type = [("float_input", FloatTensorType([None, n_features]))]
        return convert_lightgbm(model, initial_types=initial_type, target_opset=opset)
    except Exception:
        return None


def _convert_skl2onnx(model: Any, name: str, n_features: int, opset: int) -> Any:
    from skl2onnx import convert_sklearn
    from skl2onnx.common.data_types import FloatTensorType

    initial_type = [("float_input", FloatTensorType([None, n_features]))]
    return convert_sklearn(model, name=name, initial_types=initial_type, target_opset=opset)


def upload_model(object_path: str, local_path: str | None = None) -> str:
    """Upload an ONNX model to MinIO and return its URI.

    Parameters
    ----------
    object_path:
        Destination path in MinIO bucket, e.g. "strategies/momentum/v1.onnx".
    local_path:
        Local path to the ONNX file.  If None, assumed to be the same as
        *object_path* relative to CWD.

    Returns
    -------
    str
        s3:// URI of the uploaded model.
    """
    import os
    from minio import Minio

    endpoint = os.environ.get("ALFQ_MINIO_ENDPOINT", "localhost:9002")
    access_key = os.environ.get("ALFQ_MINIO_AK", "alfq")
    secret_key = os.environ.get("ALFQ_MINIO_SK", "alfq_dev")
    bucket = os.environ.get("ALFQ_MINIO_BUCKET", "alfq-models")

    client = Minio(endpoint, access_key=access_key, secret_key=secret_key, secure=False)

    if not client.bucket_exists(bucket):
        client.make_bucket(bucket)

    src = local_path or object_path
    client.fput_object(bucket, object_path, src)

    return f"s3://{bucket}/{object_path}"


def load_onnx(model_path: str) -> Any:
    """Load an ONNX model and return an ONNX Runtime inference session.

    Parameters
    ----------
    model_path:
        Local path to the ONNX file, or s3:// URI.

    Returns
    -------
    onnxruntime.InferenceSession
    """
    import onnxruntime as ort

    if model_path.startswith("s3://"):
        # Would need to download via MinIO first; for now raise
        raise NotImplementedError("s3:// download not implemented; use local path")

    return ort.InferenceSession(model_path)


def predict_onnx(session: Any, X: Any) -> Any:
    """Run inference with an ONNX Runtime session.

    Parameters
    ----------
    session:
        onnxruntime.InferenceSession from load_onnx().
    X:
        numpy array or list of lists with shape (n_samples, n_features).

    Returns
    -------
    numpy.ndarray
        Predictions.
    """
    import numpy as np

    X = np.asarray(X, dtype=np.float32)
    input_name = session.get_inputs()[0].name
    return session.run(None, {input_name: X})[0]
