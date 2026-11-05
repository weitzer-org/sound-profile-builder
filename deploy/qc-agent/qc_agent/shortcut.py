"""Create or remove the desktop shortcut "Start QC bridge" (owner's decision: the
only way the bridge starts; plan section 5).

The .lnk file is written byte by byte from the Shell Link format spec
([MS-SHLLINK]), in Python: no PowerShell, no COM, no admin rights, no Task
Scheduler, no service, no autostart. The shortcut runs

    <venv>\\Scripts\\python.exe -E -s -m qc_agent run

in deploy\\qc-agent (-E ignores PYTHON* environment variables, -s skips the user
site-packages, so only the venv and this folder supply code).

The link carries the target twice, both standard: a LinkInfo local base path, and
an EnvironmentVariableDataBlock (HasExpString), which the shell prefers when present.
"""
from __future__ import annotations

import ctypes
import os
import struct
import sys
import uuid
from pathlib import Path

SHORTCUT_NAME = "Start QC bridge.lnk"
DESCRIPTION = "Start the QC-2 Quad Cortex bridge (exits when idle)"
SW_SHOWNORMAL = 1

# [MS-SHLLINK] 2.1.1 LinkFlags
HAS_LINK_INFO = 0x00000002
HAS_NAME = 0x00000004
HAS_WORKING_DIR = 0x00000010
HAS_ARGUMENTS = 0x00000020
HAS_ICON_LOCATION = 0x00000040
IS_UNICODE = 0x00000080
HAS_EXP_STRING = 0x00000200
FILE_ATTRIBUTE_NORMAL = 0x00000080
CLSID_SHELL_LINK = uuid.UUID("00021401-0000-0000-C000-000000000046")
ENV_BLOCK_SIG = 0xA0000001
DRIVE_FIXED = 3


def _string_data(s: str) -> bytes:
    enc = s.encode("utf-16-le")
    return struct.pack("<H", len(enc) // 2) + enc


def build_lnk(target: str, arguments: str, working_dir: str, description: str = DESCRIPTION,
              icon: str | None = None) -> bytes:
    """The bytes of a .lnk pointing at `target` (an absolute Windows path)."""
    if not (len(target) > 3 and target[1:3] == ":\\"):
        raise ValueError("target must be an absolute path like C:\\...")
    flags = HAS_LINK_INFO | HAS_NAME | HAS_WORKING_DIR | HAS_ARGUMENTS | IS_UNICODE | HAS_EXP_STRING
    if icon:
        flags |= HAS_ICON_LOCATION
    header = struct.pack("<I16sIIQQQIiIHHII", 0x4C, CLSID_SHELL_LINK.bytes_le, flags, FILE_ATTRIBUTE_NORMAL,
                         0, 0, 0, 0, 0, SW_SHOWNORMAL, 0, 0, 0, 0)
    assert len(header) == 0x4C

    # LinkInfo (2.3) with VolumeIDAndLocalBasePath; the optional Unicode fields
    # (header size 0x24) carry non-ASCII paths faithfully.
    volume_id = struct.pack("<IIII", 0x11, DRIVE_FIXED, 0, 0x10) + b"\x00"
    base_ansi = target.encode("mbcs" if sys.platform == "win32" else "ascii", "replace") + b"\x00"
    suffix_ansi = b"\x00"
    base_uni = target.encode("utf-16-le") + b"\x00\x00"
    suffix_uni = b"\x00\x00"
    hdr_size = 0x24
    vol_off = hdr_size
    base_off = vol_off + len(volume_id)
    suffix_off = base_off + len(base_ansi)
    base_uni_off = suffix_off + len(suffix_ansi)
    suffix_uni_off = base_uni_off + len(base_uni)
    total = suffix_uni_off + len(suffix_uni)
    link_info = struct.pack("<IIIIIIIII", total, hdr_size, 0x1, vol_off, base_off, 0, suffix_off,
                            base_uni_off, suffix_uni_off) + volume_id + base_ansi + suffix_ansi + base_uni + suffix_uni
    assert len(link_info) == total

    strings = _string_data(description) + _string_data(working_dir) + _string_data(arguments)
    if icon:
        strings += _string_data(icon)

    # EnvironmentVariableDataBlock (2.5.4): 260-byte ANSI + 520-byte Unicode target.
    if len(target) >= 260:
        raise ValueError("target path is too long for a shortcut")
    ansi = target.encode("ascii", "replace")[:259].ljust(260, b"\x00")
    uni = target.encode("utf-16-le").ljust(520, b"\x00")
    env_block = struct.pack("<II", 0x314, ENV_BLOCK_SIG) + ansi + uni
    terminal = b"\x00\x00\x00\x00"
    return header + link_info + strings + env_block + terminal


def parse_lnk(data: bytes) -> dict:
    """Read back what build_lnk wrote (tests and `install-shortcut --check`)."""
    size, clsid, flags = struct.unpack_from("<I16sI", data, 0)
    if size != 0x4C or uuid.UUID(bytes_le=clsid) != CLSID_SHELL_LINK:
        raise ValueError("not a shell link")
    show = struct.unpack_from("<I", data, 60)[0]
    off = 0x4C
    out = {"flags": flags, "show": show}
    if flags & 0x1:
        raise ValueError("unexpected IDList")
    if flags & HAS_LINK_INFO:
        total, hdr, li_flags, _vol, base_off = struct.unpack_from("<IIIII", data, off)
        if hdr >= 0x24:
            base_uni_off = struct.unpack_from("<I", data, off + 28)[0]
            end = data.index(b"\x00\x00", off + base_uni_off)
            while (end - (off + base_uni_off)) % 2:
                end = data.index(b"\x00\x00", end + 1)
            out["target"] = data[off + base_uni_off:end].decode("utf-16-le")
        off += total
    names = [("description", HAS_NAME), ("working_dir", HAS_WORKING_DIR), ("arguments", HAS_ARGUMENTS),
             ("icon", HAS_ICON_LOCATION)]
    for key, bit in names:
        if flags & bit:
            n = struct.unpack_from("<H", data, off)[0]
            out[key] = data[off + 2:off + 2 + 2 * n].decode("utf-16-le")
            off += 2 + 2 * n
    blk_size, sig = struct.unpack_from("<II", data, off)
    if sig == ENV_BLOCK_SIG:
        uni = data[off + 8 + 260:off + 8 + 260 + 520]
        out["env_target"] = uni.decode("utf-16-le").split("\x00", 1)[0]
        off += blk_size
    if data[off:off + 4] != b"\x00\x00\x00\x00":
        raise ValueError("missing terminal block")
    return out


def desktop_dir() -> Path:
    """The user's Desktop (follows OneDrive/known-folder redirection on Windows)."""
    if sys.platform == "win32":
        folder_id = uuid.UUID("B4BFCC3A-DB2C-424C-B029-7FE99A87C641")  # FOLDERID_Desktop
        guid = (ctypes.c_byte * 16).from_buffer_copy(folder_id.bytes_le)
        path_ptr = ctypes.c_wchar_p()
        shell32 = ctypes.windll.shell32
        ole32 = ctypes.windll.ole32
        if shell32.SHGetKnownFolderPath(ctypes.byref(guid), 0, None, ctypes.byref(path_ptr)) == 0:
            try:
                return Path(path_ptr.value)
            finally:
                ole32.CoTaskMemFree(path_ptr)
    return Path.home() / "Desktop"


def agent_dir() -> Path:
    return Path(__file__).resolve().parents[1]


def install(python_exe: str | None = None, desktop: Path | None = None) -> Path:
    # python.exe, not pythonw.exe: the console window is the bridge's UI, and closing it stops the bridge.
    # No resolve(): a venv's python.exe may be a link to the base interpreter, and the venv path is what counts.
    exe = python_exe or str(Path(sys.executable).absolute())
    if sys.prefix == sys.base_prefix and python_exe is None:
        raise RuntimeError("run install-shortcut with the bridge's venv python "
                           "(e.g. %LOCALAPPDATA%\\qc-agent\\venv\\Scripts\\python.exe), not the system one")
    path = (desktop or desktop_dir()) / SHORTCUT_NAME
    data = build_lnk(exe, "-E -s -m qc_agent run", str(agent_dir()), DESCRIPTION)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(".lnk.tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)
    return path


def uninstall(desktop: Path | None = None) -> Path | None:
    path = (desktop or desktop_dir()) / SHORTCUT_NAME
    if not path.exists():
        return None
    try:
        info = parse_lnk(path.read_bytes())
    except (ValueError, struct.error, IndexError):
        raise RuntimeError(f"{path} exists but wasn't written by install-shortcut; leaving it alone") from None
    if "-m qc_agent run" not in info.get("arguments", ""):
        raise RuntimeError(f"{path} doesn't start the QC bridge; leaving it alone")
    path.unlink()
    return path
