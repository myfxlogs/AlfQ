"""Smoke test to verify the research package imports correctly."""


def test_smoke():
    import alfq_research

    assert alfq_research.__version__ == "0.1.0"
