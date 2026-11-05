"""SHA-256 of every vendored qc-mcp file, as committed upstream at f634ecd.

Checked by tests/test_vendor_pins.py and again at runtime before the transport is
imported (qcmcp_backend.load_vendored), so the agent never drives the unit with a
transport that differs from the reviewed one. A change here is a re-vendoring and
belongs in its own PR (vendor/PROVENANCE.md).
"""
from pathlib import Path

UPSTREAM = "https://github.com/lexasoft123/qc-mcp"
COMMIT = "f634ecd3dcf426090b90bc4cc176e4b500494b9e"
VENDOR_DIR = Path(__file__).resolve().parents[1] / "vendor"

# vendored path (relative to VENDOR_DIR) -> (upstream path, sha256)
PINS = {
    "qc_mcp/LICENSE": ("LICENSE", "5a8c27f4317d555055d8eb3868c23dc0e0d11ff511fd0905b617c7c254a6ebc1"),
    "qc_mcp/__init__.py": ("src/qc_mcp/__init__.py", "01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b"),
    "qc_mcp/transport.py": ("src/qc_mcp/transport.py", "9c5229946e5656edc7650f9b8c7b195d4083286137f5cd54ecc1898a3202ad3a"),
    "qc_mcp/protocol.py": ("src/qc_mcp/protocol.py", "197a447bcaba1601df659586ed28bd62b9dc640854101bc76c1c1f8fbd9cf911"),
    "qc_mcp/backend.py": ("src/qc_mcp/backend.py", "bc13d6c6921faa4bd585b1a5da7cb695f413ccb3968cfa7c9fa1bf31b52afa55"),
    "qc_mcp/winhid.py": ("src/qc_mcp/winhid.py", "5359005ecb5163455fcfaad09ed50b827e1765faf0b83df986d2bfbfe32235ab"),
    "qc_mcp/directory.py": ("src/qc_mcp/directory.py", "ddacedf0f58521b57d643887509ec1dca1f18b434213944f57e43b9d83593205"),
    "qc_mcp/descriptors/qc_descriptors-4.0.pb": ("src/qc_mcp/descriptors/qc_descriptors-4.0.pb",
                                                "fe672cdd85886bac0ccae31920c7dd8a7650aaaaeaedb3301e55663e0fffdfb3"),
    "qc_mcp/descriptors/qc_descriptors-4.1.pb": ("src/qc_mcp/descriptors/qc_descriptors-4.1.pb",
                                                "eed223846169c1beb7a62b76c288d04d541fd1d5fd1853d0a2d8f68660f530c7"),
}


UNPINNED_OK = frozenset({"PROVENANCE.md", ".gitattributes"})


def mismatches(vendor_dir: Path = VENDOR_DIR) -> list[str]:
    """Vendored files that are missing or differ from their pin, plus unexpected extra files."""
    import hashlib
    bad = []
    for rel, (_up, want) in PINS.items():
        p = vendor_dir / rel
        try:
            got = hashlib.sha256(p.read_bytes()).hexdigest()
        except OSError:
            bad.append(f"{rel}: missing")
            continue
        if got != want:
            bad.append(f"{rel}: sha256 {got[:12]}..., pinned {want[:12]}...")
    if vendor_dir.is_dir():
        # The whole tree, __pycache__ included: an extra module (e.g. vendor/google/) or a planted .pyc
        # must not hide behind green pins. The agent loads the vendored modules from their hashed bytes
        # (qcmcp_backend.load_vendored), so it never writes bytecode here itself.
        for p in vendor_dir.rglob("*"):
            rel = p.relative_to(vendor_dir).as_posix()
            if p.is_file() and rel not in PINS and rel not in UNPINNED_OK:
                bad.append(f"{rel}: not a pinned file")
    return bad


def verified_bytes(rel: str, vendor_dir: Path = VENDOR_DIR) -> bytes:
    """A pinned file's bytes, read once and checked against its pin (no gap between check and use)."""
    import hashlib
    data = (vendor_dir / rel).read_bytes()
    if hashlib.sha256(data).hexdigest() != PINS[rel][1]:
        raise ValueError(f"{rel} differs from its f634ecd pin")
    return data
