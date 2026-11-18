"""Minimal retrying HTTP client built on urllib."""

import http.client
import socket
import time
import urllib.error
import urllib.parse
import urllib.request

UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/120.0 Safari/537.36")


# --------------------------------------------------------------------------- #
# HTTP helpers
# --------------------------------------------------------------------------- #
def _http(method, url, body=None, headers=None, timeout=30, retries=2):
    """HTTP 请求。对 Telegram 边缘偶发的 400 做重试。

    Telegram 的 api.telegram.org 边缘前置（nginx）会偶发对完全合法的请求返回
    400（同 IP、同 token、同 payload 的裸 socket 请求同时却是 200）。命中时
    通知会静默丢失。这里对 400/5xx 做有限重试，间隔递增；仍失败则抛出。
    """
    h = {"User-Agent": UA}
    if headers:
        h.update(headers)
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        h["Content-Type"] = "application/json"
    last = None
    for attempt in range(retries + 1):
        req = urllib.request.Request(url, data=data, method=method, headers=h)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as r:
                raw = r.read().decode("utf-8", errors="ignore")
                return r.status, raw
        except urllib.error.HTTPError as e:
            last = e
            # 4xx 里只有 400/408/429 值得重试；其余（如 404/403）立即失败
            if e.code not in (400, 408, 429) and e.code < 500:
                raise
            if attempt < retries:
                time.sleep(0.5 * (attempt + 1))
    if last is not None:
        raise last
    raise RuntimeError("_http: no attempt made")
