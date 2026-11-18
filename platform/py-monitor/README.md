# py-monitor

Stdlib-only primitives extracted from the Carousell new-listing monitor
(`hoelee/carousell-monitor`), kept here because the same URL normalisation,
price parsing and retry logic is needed by every scraper in the stack.

No third-party dependencies - `urllib` only, so it drops into any container
without a build step.

```python
from monitor_core import normalize_product_url, parse_price, strip_thumbnail_suffix
```

Run the tests with `python -m pytest tests/ -q` (or `python tests/test_core.py`).
