"""Tests for ALFQ backtest engine."""
from alfq_research.backtest import BacktestConfig, Trade, BacktestResult


def test_backtest_config_defaults():
    cfg = BacktestConfig()
    assert cfg.initial_capital == 100_000.0
    assert cfg.commission_per_lot == 7.0
    assert cfg.slippage_points_mean == 0.0


def test_backtest_config_custom():
    cfg = BacktestConfig(initial_capital=50_000, commission_per_lot=5.0)
    assert cfg.initial_capital == 50_000
    assert cfg.commission_per_lot == 5.0


def test_trade_creation():
    trade = Trade(symbol="EURUSD", side="buy", entry_price=1.05, volume=0.1, entry_ts_ms=1700000000000)
    assert trade.symbol == "EURUSD"
    assert trade.side == "buy"
    assert trade.entry_price == 1.05
    assert trade.pnl == 0.0


def test_trade_with_pnl():
    trade = Trade(symbol="XAUUSD", side="sell", entry_price=2000.0, exit_price=1995.0, volume=1.0, pnl=500.0)
    assert trade.pnl == 500.0
    assert trade.exit_price == 1995.0


def test_backtest_result_empty():
    result = BacktestResult()
    assert result.trades == []
    assert result.equity_curve == []
    assert result.total_return == 0.0
    assert result.sharpe_ratio == 0.0
    assert result.max_drawdown == 0.0


def test_backtest_result_with_trades():
    trades = [
        Trade(symbol="EURUSD", side="buy", entry_price=1.05, exit_price=1.06, volume=0.1, pnl=100.0),
        Trade(symbol="GBPUSD", side="sell", entry_price=1.25, exit_price=1.26, volume=0.2, pnl=-200.0),
    ]
    result = BacktestResult(trades=trades, total_return=-100.0, win_rate=0.5)
    assert len(result.trades) == 2
    assert result.total_return == -100.0
    assert result.win_rate == 0.5
