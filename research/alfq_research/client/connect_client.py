"""ALFQ Connect RPC client — lightweight httpx-based Connect client.

Calls trading-core Connect endpoints with JWT auth.  No code generation
dependency — uses plain JSON/HTTP which is compatible with Connect's
JSON-mode protocol.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field

import httpx
from loguru import logger

from .auth import get_token


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


@dataclass
class ConnectClient:
    """Thin Connect RPC client for trading-core API.

    Environment:
        ALFQ_API_BASE  — e.g. http://localhost:9000
        ALFQ_API_TOKEN — JWT access token (optional if using auth flow)
        ALFQ_TENANT_ID — tenant identifier
    """

    base_url: str = field(default_factory=lambda: _env("ALFQ_API_BASE", "http://localhost:9000"))
    token: str = field(default_factory=lambda: _env("ALFQ_API_TOKEN", ""))
    tenant_id: str = field(default_factory=lambda: _env("ALFQ_TENANT_ID", ""))
    timeout: float = 30.0

    _client: httpx.Client | None = field(default=None, init=False, repr=False)

    @property
    def client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(
                base_url=self.base_url.rstrip("/"),
                timeout=self.timeout,
                headers=self._headers(),
            )
        return self._client

    def _headers(self) -> dict[str, str]:
        h: dict[str, str] = {"Content-Type": "application/json"}
        tok = self.token or get_token()
        if tok:
            h["Authorization"] = f"Bearer {tok}"
        if self.tenant_id:
            h["X-Tenant-Id"] = self.tenant_id
        return h

    # ── Strategy ──

    def submit_strategy(self, spec, override_name: str = "") -> dict[str, object]:
        """Submit a StrategySpec to trading-core.

        POST /alfq.v1.StrategyService/CreateStrategy
        """
        payload = spec.to_dict()
        if override_name:
            payload["name"] = override_name

        try:
            resp = self.client.post(
                "/alfq.v1.StrategyService/CreateStrategy",
                json=payload,
            )
            resp.raise_for_status()
            return resp.json()
        except httpx.HTTPStatusError as exc:
            logger.error("Strategy submit failed: {} {}", exc.response.status_code, exc.response.text)
            raise

    def list_strategies(self) -> dict[str, object]:
        """List strategies for current tenant.

        POST /alfq.v1.StrategyService/ListStrategies
        """
        resp = self.client.post(
            "/alfq.v1.StrategyService/ListStrategies",
            json={},
        )
        resp.raise_for_status()
        return resp.json()

    def get_strategy(self, strategy_id: str) -> dict[str, object]:
        resp = self.client.post(
            "/alfq.v1.StrategyService/GetStrategy",
            json={"strategy_id": strategy_id},
        )
        resp.raise_for_status()
        return resp.json()

    # ── Backtest ──

    def start_backtest(self, strategy_id: str, params: dict | None = None) -> dict[str, object]:
        """Start a backtest job.

        POST /alfq.v1.BacktestService/StartBacktest
        """
        resp = self.client.post(
            "/alfq.v1.BacktestService/StartBacktest",
            json={"strategy_id": strategy_id, "params": params or {}},
        )
        resp.raise_for_status()
        return resp.json()

    def get_backtest_result(self, backtest_id: str) -> dict[str, object]:
        resp = self.client.post(
            "/alfq.v1.BacktestService/GetBacktestResult",
            json={"backtest_id": backtest_id},
        )
        resp.raise_for_status()
        return resp.json()

    # ── Factor ──

    def create_factor(self, name: str, expression: str, description: str = "") -> dict[str, object]:
        resp = self.client.post(
            "/alfq.v1.FactorService/CreateFactor",
            json={"name": name, "expression": expression, "description": description},
        )
        resp.raise_for_status()
        return resp.json()

    # ── Symbol ──

    def list_broker_symbols(self, broker_id: str) -> dict[str, object]:
        resp = self.client.post(
            "/alfq.v1.SymbolService/ListBrokerSymbols",
            json={"broker_id": broker_id},
        )
        resp.raise_for_status()
        return resp.json()

    def resolve_symbol(self, account_id: str, canonical: str) -> dict[str, object]:
        resp = self.client.post(
            "/alfq.v1.SymbolService/ResolveSymbol",
            json={"account_id": account_id, "canonical": canonical},
        )
        resp.raise_for_status()
        return resp.json()


# ── Singleton access ──

_client: ConnectClient | None = None


def get_client() -> ConnectClient:
    global _client
    if _client is None:
        _client = ConnectClient()
    return _client
