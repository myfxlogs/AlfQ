"""ALFQ JWT auth helpers for research client."""

from __future__ import annotations

import os

_TOKEN: str | None = None


def get_token() -> str:
    """Return the current JWT token.

    Order of precedence:
    1. ALFQ_API_TOKEN environment variable
    2. ~/.alfq/token file
    """
    global _TOKEN
    if _TOKEN:
        return _TOKEN

    token = os.environ.get("ALFQ_API_TOKEN", "")
    if token:
        _TOKEN = token
        return token

    token_file = os.path.expanduser("~/.alfq/token")
    try:
        with open(token_file) as f:
            token = f.read().strip()
            if token:
                _TOKEN = token
                return token
    except FileNotFoundError:
        pass

    return ""


def set_token(token: str) -> None:
    """Set the JWT token in memory."""
    global _TOKEN
    _TOKEN = token
