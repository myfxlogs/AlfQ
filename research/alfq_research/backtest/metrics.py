"""ALFQ Backtest Metrics — Sharpe, Sortino, MaxDD, Calmar."""
import math


def sharpe_ratio(returns: list[float], rf: float = 0.0) -> float:
    if len(returns) < 2: return 0
    excess = [r - rf/252 for r in returns]
    mean = sum(excess)/len(excess)
    std = math.sqrt(sum((x-mean)**2 for x in excess)/(len(excess)-1))
    return (mean/std)*math.sqrt(252) if std>0 else 0

def max_drawdown(equity: list[float]) -> float:
    peak = equity[0]; max_dd = 0
    for v in equity:
        if v>peak: peak=v
        dd=(peak-v)/peak
        if dd>max_dd: max_dd=dd
    return max_dd

def sortino_ratio(returns: list[float], rf: float = 0.0) -> float:
    if len(returns) < 2: return 0
    excess = [r - rf/252 for r in returns]
    mean = sum(excess)/len(excess)
    downside = [min(0,x) for x in excess]
    std_down = math.sqrt(sum(x**2 for x in downside)/(len(downside)-1))
    return (mean/std_down)*math.sqrt(252) if std_down>0 else 0

def calmar_ratio(returns: list[float], equity: list[float]) -> float:
    annual_return = sum(returns)/len(returns)*252 if returns else 0
    dd = max_drawdown(equity)
    return annual_return/dd if dd>0 else 0
