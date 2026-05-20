"""ALFQ StrategySpec — strategy definition for submission to trading-core.

A StrategySpec defines a complete trading strategy: canonical symbols, factor
expressions, model reference, signal rule, sizing, and risk parameters.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class StrategySpec:
    """Complete strategy specification.

    Canonical symbols only — the broker-level symbol_raw translation is done
    by trading-core at deployment time via broker_symbols table lookup.
    """

    name: str = ""
    version: str = "1.0.0"
    canonical_symbols: list[str] = field(default_factory=list)
    period: str = "1h"
    factors: dict[str, str] = field(default_factory=dict)
    signal_rule: str = ""
    model_uri: str = ""                # s3:// bucket / path to ONNX model
    model_inputs: list[str] = field(default_factory=list)
    sizing: dict[str, Any] = field(default_factory=dict)
    risk_limits: dict[str, float] = field(default_factory=dict)
    description: str = ""

    # ── Validation ──

    def validate(self) -> list[str]:
        """Validate the spec and return a list of issues (empty = valid)."""
        issues: list[str] = []
        if not self.name.strip():
            issues.append("name is required")
        if not self.canonical_symbols:
            issues.append("canonical_symbols is required")
        if not self.signal_rule.strip() and not self.model_uri:
            issues.append("either signal_rule or model_uri is required")
        if self.model_uri and not self.model_inputs:
            issues.append("model_inputs is required when model_uri is set")
        if self.period not in ("1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w"):
            issues.append(f"unknown period: {self.period}")
        return issues

    def is_valid(self) -> bool:
        return len(self.validate()) == 0

    # ── Serialisation ──

    def to_dict(self) -> dict[str, object]:
        return {
            "name": self.name,
            "version": self.version,
            "canonical_symbols": self.canonical_symbols,
            "period": self.period,
            "factors": self.factors,
            "signal_rule": self.signal_rule,
            "model_uri": self.model_uri,
            "model_inputs": self.model_inputs,
            "sizing": self.sizing,
            "risk_limits": self.risk_limits,
            "description": self.description,
        }

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> StrategySpec:
        return cls(
            name=data.get("name", ""),
            version=data.get("version", "1.0.0"),
            canonical_symbols=data.get("canonical_symbols", []),
            period=data.get("period", "1h"),
            factors=data.get("factors", {}),
            signal_rule=data.get("signal_rule", ""),
            model_uri=data.get("model_uri", ""),
            model_inputs=data.get("model_inputs", []),
            sizing=data.get("sizing", {}),
            risk_limits=data.get("risk_limits", {}),
            description=data.get("description", ""),
        )

    @classmethod
    def from_json(cls, raw: str) -> StrategySpec:
        return cls.from_dict(json.loads(raw))

    @classmethod
    def from_yaml(cls, path: str | Path) -> StrategySpec:
        import yaml
        with open(path) as f:
            return cls.from_dict(yaml.safe_load(f))

    def to_yaml(self, path: str | Path) -> None:
        import yaml
        with open(path, "w") as f:
            yaml.safe_dump(self.to_dict(), f, default_flow_style=False)

    # ── Submission ──

    def submit(self, name: str = "") -> dict[str, object]:
        """Submit this spec to trading-core via Connect RPC.

        Returns the API response dict.
        """
        from alfq_research.client.connect_client import get_client

        client = get_client()
        return client.submit_strategy(self, override_name=name)
