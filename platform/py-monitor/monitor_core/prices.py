"""Price and condition parsing."""

import re

CONDITIONS = ["Brand new", "Like new", "Lightly used", "Well used", "Heavily used", "Used"]
CONDITION_MAP = {
    "brand new": "Brand new",
    "new": "Brand new",
    "like new": "Like new",
    "lightly used": "Lightly used",
    "well used": "Well used",
    "heavily used": "Heavily used",
    "used": "Used",
}


def parse_price(s):
    if not s:
        return None
    t = re.sub(r"[^0-9.]", "", s)
    if not t:
        return None
    try:
        return round(float(t), 2)
    except ValueError:
        return None


def normalize_condition(value):
    """Map a category-specific condition label onto the canonical set."""
    if not value:
        return None
    return CONDITION_MAP.get(str(value).strip().lower())
