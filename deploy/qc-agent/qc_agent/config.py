"""Load %LOCALAPPDATA%\\qc-agent\\agent.env (written by deploy/qc-agent/write_agent_env.py).

The variable names are the configuration contract with that script and with
docs/design/qc-device-bridge-plan.md section 5. Only the file is read: process
environment variables never override it, so what runs is exactly what the owner
wrote. Nothing in this module logs a secret; Config's repr redacts them.
"""
from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from pathlib import Path

from . import contract
from .slots import SlotRanges

REQUIRED = (
    "QC_AGENT_R2_ENDPOINT", "QC_AGENT_R2_BUCKET",
    "QC_AGENT_R2_ACCESS_KEY_ID", "QC_AGENT_R2_SECRET_ACCESS_KEY",
    "DEVICE_JOB_KEY",
    "QC_AGENT_ALLOWED_SLOTS", "QC_AGENT_EXPECTED_FIRMWARE", "QC_AGENT_IDLE_MINUTES",
)
OPTIONAL = ("DEVICE_JOB_KEY_NEXT", "QC_AGENT_EDIT_MODES", "QC_AGENT_PROTECTED_SLOTS")
# Preset editing (docs/design/qc-preset-editing.md section 6). Both settings live on this PC so a signed job
# alone can never switch editing on: QC_AGENT_EDIT_MODES lists the save modes the owner allows (empty = every
# edit is refused), QC_AGENT_PROTECTED_SLOTS the slots no edit ever touches.
EDIT_MODES = ("live", "copy", "overwrite")
KNOWN = frozenset(REQUIRED + OPTIONAL)

_LINE_RE = re.compile(r"([A-Z][A-Z0-9_]*)=(.*)")
_ENDPOINT_RE = re.compile(r"https://[A-Za-z0-9.-]+(?::[0-9]{1,5})?")
_FIRMWARE_RE = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+")


class ConfigError(ValueError):
    """agent.env is missing, malformed, or holds an invalid value. The message never
    contains a secret value."""


def default_home() -> Path:
    base = os.environ.get("LOCALAPPDATA")
    if base:
        return Path(base) / "qc-agent"
    return Path.home() / ".qc-agent"  # non-Windows: tests and development only


@dataclass(frozen=True)
class Config:
    endpoint: str
    bucket: str
    access_key_id: str
    secret_access_key: str = field(repr=False)
    keys: dict = field(repr=False)          # {kid: key_bytes}, current first
    current_kid: str
    allowed_slots: SlotRanges
    expected_firmware: str
    idle_minutes: int
    edit_modes: frozenset = frozenset()          # QC_AGENT_EDIT_MODES; empty = edit_preset is refused
    protected_slots: SlotRanges | None = None    # QC_AGENT_PROTECTED_SLOTS; None = no slot is protected

    @property
    def signing_key(self) -> bytes:
        return self.keys[self.current_kid]

    def __repr__(self):  # belt and braces: dataclass repr already skips the secrets
        return (f"Config(endpoint={self.endpoint!r}, bucket={self.bucket!r}, "
                f"access_key_id=<{len(self.access_key_id)} chars>, current_kid={self.current_kid!r}, "
                f"allowed_slots={self.allowed_slots}, expected_firmware={self.expected_firmware!r}, "
                f"idle_minutes={self.idle_minutes}, edit_modes={sorted(self.edit_modes)}, "
                f"protected_slots={self.protected_slots})")


def parse_env_text(text: str) -> dict[str, str]:
    """KEY=VALUE lines; blank lines and '#' comments ignored. Strict: unknown keys,
    duplicates and malformed lines are errors (a typo must not silently drop a setting)."""
    out: dict[str, str] = {}
    for n, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        m = _LINE_RE.fullmatch(line)
        if not m:
            raise ConfigError(f"agent.env line {n}: expected NAME=value")
        name, value = m.group(1), m.group(2).strip()
        if name not in KNOWN:
            raise ConfigError(f"agent.env line {n}: unknown setting {name}")
        if name in out:
            raise ConfigError(f"agent.env line {n}: {name} is set twice")
        out[name] = value
    return out


def _parse_edit_modes(value) -> frozenset:
    """QC_AGENT_EDIT_MODES: a comma-separated subset of live, copy, overwrite. Unset or empty = none (every
    edit refused). A typo is an error, never a silently narrower or wider list."""
    parts = [p.strip() for p in (value or "").split(",") if p.strip()]
    bad = [p for p in parts if p not in EDIT_MODES]
    if bad:
        raise ConfigError("QC_AGENT_EDIT_MODES lists only " + ", ".join(EDIT_MODES) + " (comma-separated); "
                          "leave it empty to refuse every edit")
    return frozenset(parts)


def _parse_protected(value) -> SlotRanges | None:
    if not (value or "").strip():
        return None
    try:
        return SlotRanges.parse(value.strip())
    except ValueError as e:
        raise ConfigError(f"QC_AGENT_PROTECTED_SLOTS: {e}") from None


def from_mapping(env: dict[str, str]) -> Config:
    missing = [k for k in REQUIRED if not env.get(k)]
    if missing:
        raise ConfigError("agent.env is missing: " + ", ".join(missing)
                          + ". Re-run deploy\\qc-agent\\write_agent_env.py.")
    endpoint = env["QC_AGENT_R2_ENDPOINT"].rstrip("/")
    if not _ENDPOINT_RE.fullmatch(endpoint):
        raise ConfigError("QC_AGENT_R2_ENDPOINT must look like https://<account-id>.r2.cloudflarestorage.com")
    bucket = env["QC_AGENT_R2_BUCKET"]
    if not contract.BUCKET_RE.fullmatch(bucket):
        raise ConfigError("QC_AGENT_R2_BUCKET is not a valid bucket name")
    for name in ("QC_AGENT_R2_ACCESS_KEY_ID", "QC_AGENT_R2_SECRET_ACCESS_KEY"):
        v = env[name]
        if any(c.isspace() or ord(c) < 0x21 or ord(c) > 0x7E for c in v):
            raise ConfigError(f"{name} contains spaces or non-printable characters")
    nxt = (env.get("DEVICE_JOB_KEY_NEXT") or "").strip()
    if nxt and not contract.KEY_RE.fullmatch(nxt):
        raise ConfigError("DEVICE_JOB_KEY_NEXT must be exactly 64 hex characters")
    try:
        keys = contract.keyring(env["DEVICE_JOB_KEY"], env.get("DEVICE_JOB_KEY_NEXT"))
    except ValueError as e:  # the reference's messages name the variable, never the value
        raise ConfigError(str(e)) from None
    current_kid = contract.kid(contract.parse_key(env["DEVICE_JOB_KEY"]))
    try:
        slots = SlotRanges.parse(env["QC_AGENT_ALLOWED_SLOTS"])
    except ValueError as e:
        raise ConfigError(f"QC_AGENT_ALLOWED_SLOTS: {e}") from None
    firmware = env["QC_AGENT_EXPECTED_FIRMWARE"]
    if not _FIRMWARE_RE.fullmatch(firmware):
        raise ConfigError("QC_AGENT_EXPECTED_FIRMWARE must be an exact version such as 4.1.0")
    idle = env["QC_AGENT_IDLE_MINUTES"]
    if not re.fullmatch(r"[0-9]{1,3}", idle) or not 1 <= int(idle) <= 240:
        raise ConfigError("QC_AGENT_IDLE_MINUTES must be a whole number of minutes, 1-240")
    modes = _parse_edit_modes(env.get("QC_AGENT_EDIT_MODES"))
    protected = _parse_protected(env.get("QC_AGENT_PROTECTED_SLOTS"))
    return Config(endpoint=endpoint, bucket=bucket,
                  access_key_id=env["QC_AGENT_R2_ACCESS_KEY_ID"],
                  secret_access_key=env["QC_AGENT_R2_SECRET_ACCESS_KEY"],
                  keys=keys, current_kid=current_kid, allowed_slots=slots,
                  expected_firmware=firmware, idle_minutes=int(idle),
                  edit_modes=modes, protected_slots=protected)


def load(path: Path) -> Config:
    try:
        with open(path, "rb") as f:
            raw = f.read(64 * 1024 + 1)
    except FileNotFoundError:
        raise ConfigError(f"{path} does not exist. Run deploy\\qc-agent\\write_agent_env.py first.") from None
    except OSError as e:
        raise ConfigError(f"cannot read {path}: {e.strerror or type(e).__name__}") from None
    if len(raw) > 64 * 1024:
        raise ConfigError(f"{path} is unexpectedly large")
    try:
        text = raw.decode("utf-8-sig")
    except UnicodeDecodeError:
        raise ConfigError(f"{path} is not UTF-8 text") from None
    return from_mapping(parse_env_text(text))
