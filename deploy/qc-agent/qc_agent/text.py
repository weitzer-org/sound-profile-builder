"""Untrusted text from the unit (contract section 11): strip control and format
characters and line breaks, replace backticks, collapse whitespace, bound the length."""
from __future__ import annotations

import unicodedata


def clean(s, limit: int) -> str:
    if not isinstance(s, str):
        return ""
    out = []
    for ch in s:
        cat = unicodedata.category(ch)
        if cat in ("Cc", "Cf", "Cs", "Co", "Zl", "Zp"):
            out.append(" ")
        elif ch == "`":
            out.append("'")
        else:
            out.append(ch)
    return " ".join("".join(out).split())[:limit]
