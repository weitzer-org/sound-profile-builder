"""Minimal S3 client for the jobs bucket (Cloudflare R2), standard library only.

Five operations, all path-style and signed with AWS Signature V4 (region "auto"
for R2): ListObjectsV2, a bounded ranged GET, HEAD, PUT and DELETE. Keeping this
in the standard library keeps the agent's hash-pinned lockfile to one package
(protobuf). Never logs credentials, signatures or object bodies.

Bounded reads (contract section 3): the size is taken from the listing first, and
the GET itself asks for at most cap + 1 bytes with a Range header and reads at
most that many, so an oversized object never reaches memory.
"""
from __future__ import annotations

import datetime as _dt
import hashlib
import hmac
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from typing import Protocol

EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()
_S3_NS = "{http://s3.amazonaws.com/doc/2006-03-01/}"
MAX_LIST_RESPONSE = 2 * 1024 * 1024
TIMEOUT_S = 30


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    """A redirect would re-send the signed Authorization header to wherever it points.
    R2 doesn't redirect; refuse instead of following."""

    def redirect_request(self, *args, **kwargs):
        return None


class S3Error(Exception):
    def __init__(self, op: str, status: int | None, code: str = ""):
        super().__init__(f"{op}: HTTP {status} {code}".strip() if status else f"{op}: {code}")
        self.status, self.code = status, code


def _hmac(key: bytes, msg: str) -> bytes:
    return hmac.new(key, msg.encode("utf-8"), hashlib.sha256).digest()


def _q(s: str, safe: str = "-_.~") -> str:
    return urllib.parse.quote(s, safe=safe)


def canonical_query(params: dict[str, str]) -> str:
    return "&".join(f"{_q(k)}={_q(v)}" for k, v in sorted(params.items()))


def sign_v4(method: str, host: str, path: str, query: dict[str, str], headers: dict[str, str],
            payload_sha256: str, access_key: str, secret_key: str, region: str, service: str,
            now: _dt.datetime) -> dict[str, str]:
    """Return the headers to send (including Authorization). `path` is already the
    canonical, URI-encoded path. `headers` are extra headers to sign (lower-case names)."""
    amz_date = now.strftime("%Y%m%dT%H%M%SZ")
    day = now.strftime("%Y%m%d")
    hdrs = {k.lower(): " ".join(str(v).strip().split()) for k, v in headers.items()}
    hdrs["host"] = host
    hdrs["x-amz-date"] = amz_date
    hdrs["x-amz-content-sha256"] = payload_sha256
    names = sorted(hdrs)
    canonical = "\n".join([
        method, path, canonical_query(query),
        "".join(f"{n}:{hdrs[n]}\n" for n in names),
        ";".join(names), payload_sha256,
    ])
    scope = f"{day}/{region}/{service}/aws4_request"
    to_sign = "\n".join(["AWS4-HMAC-SHA256", amz_date, scope,
                         hashlib.sha256(canonical.encode("utf-8")).hexdigest()])
    k = _hmac(_hmac(_hmac(_hmac(("AWS4" + secret_key).encode("utf-8"), day), region), service), "aws4_request")
    sig = hmac.new(k, to_sign.encode("utf-8"), hashlib.sha256).hexdigest()
    out = {n: hdrs[n] for n in names if n != "host"}
    out["authorization"] = (f"AWS4-HMAC-SHA256 Credential={access_key}/{scope}, "
                            f"SignedHeaders={';'.join(names)}, Signature={sig}")
    return out


class JobStore(Protocol):
    """What the agent needs from the jobs bucket. S3Store (R2) and MemoryStore (tests) implement it;
    every method raises S3Error on failure."""

    bucket: str

    def list_prefix(self, prefix: str, limit: int) -> tuple[list[tuple[str, int]], bool]: ...

    def get_bounded(self, key: str, cap: int) -> bytes | None: ...

    def exists(self, key: str) -> bool: ...

    def put(self, key: str, body: bytes) -> None: ...

    def delete(self, key: str) -> None: ...


class S3Store:
    """The jobs bucket over HTTPS. All methods raise S3Error on failure."""

    def __init__(self, endpoint: str, bucket: str, access_key: str, secret_key: str,
                 region: str = "auto", opener=None):
        u = urllib.parse.urlparse(endpoint)
        if u.scheme != "https" or not u.netloc or u.path not in ("", "/"):
            raise ValueError("endpoint must be https://host")
        self._base = f"https://{u.netloc}"
        self._host = u.netloc
        self.bucket = bucket
        self._ak, self._sk, self._region = access_key, secret_key, region
        self._open = opener or urllib.request.build_opener(_NoRedirect()).open

    def __repr__(self):
        return f"S3Store(host={self._host!r}, bucket={self.bucket!r})"

    def _path(self, key: str = "") -> str:
        return "/" + _q(self.bucket) + ("/" + _q(key, safe="/-_.~") if key else "")

    def _request(self, op, method, key="", query=None, body=b"", extra=None, max_read=0):
        query = query or {}
        payload_hash = hashlib.sha256(body).hexdigest()
        path = self._path(key)
        headers = sign_v4(method, self._host, path, query, extra or {}, payload_hash,
                          self._ak, self._sk, self._region, "s3",
                          _dt.datetime.now(_dt.timezone.utc))
        url = self._base + path + (("?" + canonical_query(query)) if query else "")
        req = urllib.request.Request(url, data=body if method == "PUT" else None, method=method,
                                     headers=headers)
        try:
            with self._open(req, timeout=TIMEOUT_S) as resp:
                data = resp.read(max_read + 1) if max_read else b""
                return resp.status, data
        except urllib.error.HTTPError as e:
            code = ""
            try:
                raw = e.read(4096) or b""
                start, end = raw.find(b"<Code>"), raw.find(b"</Code>")
                if 0 <= start < end:
                    code = raw[start + 6:end].decode("ascii", "replace")[:64]
            except Exception:  # noqa: BLE001
                pass
            return e.code, code
        except OSError as e:
            # The reason ("connection refused", a TLS error) helps the owner; it never holds credentials.
            reason = getattr(e, "reason", None)
            raise S3Error(op, None, f"{type(e).__name__} {reason}"[:160] if reason else type(e).__name__) from None

    # ---- operations ----

    def list_prefix(self, prefix: str, limit: int) -> tuple[list[tuple[str, int]], bool]:
        """Up to `limit` (key, size) pairs under prefix, in key order, across pages.
        Returns (items, more): more is True when the bucket held more than `limit`."""
        items: list[tuple[str, int]] = []
        token = None
        while True:
            q = {"list-type": "2", "prefix": prefix, "max-keys": str(min(1000, limit - len(items) + 1))}
            if token:
                q["continuation-token"] = token
            status, data = self._request("list", "GET", query=q, max_read=MAX_LIST_RESPONSE)
            if status != 200:
                raise S3Error("list", status, data if isinstance(data, str) else "")
            if len(data) > MAX_LIST_RESPONSE:
                raise S3Error("list", status, "response too large")
            if b"<!DOCTYPE" in data or b"<!ENTITY" in data:
                raise S3Error("list", status, "unexpected DTD in response")
            try:
                root = ET.fromstring(data)
            except ET.ParseError:
                raise S3Error("list", status, "unparseable response") from None
            for c in root.iter(_S3_NS + "Contents"):
                k, s = c.findtext(_S3_NS + "Key"), c.findtext(_S3_NS + "Size")
                if k is None or s is None or not s.isdigit():
                    raise S3Error("list", status, "malformed entry")
                items.append((k, int(s)))
                if len(items) > limit:
                    return items[:limit], True
            truncated = (root.findtext(_S3_NS + "IsTruncated") or "").lower() == "true"
            token = root.findtext(_S3_NS + "NextContinuationToken")
            if not truncated or not token:
                return items, False

    def get_bounded(self, key: str, cap: int) -> bytes | None:
        """The object's bytes, or None if it is larger than cap. Transfers at most cap+1 bytes."""
        status, data = self._request("get", "GET", key, extra={"range": f"bytes=0-{cap}"}, max_read=cap + 1)
        if status == 416:  # Range on an empty object
            return b""
        if status == 404:
            raise S3Error("get", 404, "NoSuchKey")
        if status not in (200, 206):
            raise S3Error("get", status, data if isinstance(data, str) else "")
        return None if len(data) > cap else data

    def exists(self, key: str) -> bool:
        status, data = self._request("head", "HEAD", key)
        if status == 200:
            return True
        if status == 404:
            return False
        raise S3Error("head", status, data if isinstance(data, str) else "")

    def put(self, key: str, body: bytes) -> None:
        status, data = self._request("put", "PUT", key, body=body,
                                     extra={"content-type": "application/json"})
        if status not in (200, 201):
            raise S3Error("put", status, data if isinstance(data, str) else "")

    def delete(self, key: str) -> None:
        status, data = self._request("delete", "DELETE", key)
        if status not in (200, 204, 404):
            raise S3Error("delete", status, data if isinstance(data, str) else "")


class MemoryStore:
    """In-memory stand-in for S3Store (tests). Same methods and bounds."""

    def __init__(self, bucket: str = "test-bucket"):
        self.bucket = bucket
        self.objects: dict[str, bytes] = {}
        self.ops: list[tuple[str, str]] = []
        self.fail: dict[str, Exception] = {}

    def _maybe_fail(self, op):
        if op in self.fail:
            raise self.fail[op]

    def list_prefix(self, prefix, limit):
        self._maybe_fail("list")
        self.ops.append(("list", prefix))
        keys = sorted(k for k in self.objects if k.startswith(prefix))
        return [(k, len(self.objects[k])) for k in keys[:limit]], len(keys) > limit

    def get_bounded(self, key, cap):
        self._maybe_fail("get")
        self.ops.append(("get", key))
        if key not in self.objects:
            raise S3Error("get", 404, "NoSuchKey")
        data = self.objects[key][:cap + 1]
        return None if len(data) > cap else data

    def exists(self, key):
        self._maybe_fail("head")
        self.ops.append(("head", key))
        return key in self.objects

    def put(self, key, body):
        self._maybe_fail("put")
        self.ops.append(("put", key))
        self.objects[key] = bytes(body)

    def delete(self, key):
        self._maybe_fail("delete")
        self.ops.append(("delete", key))
        self.objects.pop(key, None)
