"""Ignored-keyword matching."""


def title_matches_keyword(title, ignored_keywords):
    """True when the title contains any ignored keyword (case-insensitive).

    Keywords are lowercased here rather than by the caller: the original caller
    normalised on load, which made the helper silently order-dependent.
    """
    if not title or not ignored_keywords:
        return False
    t = str(title).lower()
    return any(str(kw).lower().strip() in t for kw in ignored_keywords if str(kw).strip())
