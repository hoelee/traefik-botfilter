"""Listing URL normalisation."""

import urllib.parse

PRODUCT_URL_TMPL = "https://www.carousell.com.my/p/{listing_id}/"


def strip_thumbnail_suffix(url):
    """去掉 Carousell 缩略图后缀 _progressive_thumbnail，得到高清原图 URL。

    两种形态都去掉：
      1) ..._progressive_thumbnail.jpg  →  ...jpg
      2) ..._progressive_thumbnail       →  ...（无扩展名，仍是合法 JPEG）
    """
    if not url:
        return url
    return url.replace("_progressive_thumbnail", "")


def normalize_product_url(url):
    """剥掉 query/fragment：同一个商品被两个 watch 抓到、或带追踪后缀时仍算同一条。"""
    if not url:
        return ""
    try:
        p = urllib.parse.urlsplit(str(url).strip())
        path = (p.path or "").rstrip("/")
        return urllib.parse.urlunsplit((p.scheme.lower(), p.netloc.lower(), path, "", ""))
    except Exception:
        return str(url).strip()
