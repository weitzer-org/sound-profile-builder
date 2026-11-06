"""Command line for the QC bridge home agent.

    python -m qc_agent run                  start the bridge (what the desktop shortcut runs)
    python -m qc_agent check                offline self-check: config, vendored transport, catalog
    python -m qc_agent install-shortcut     put "Start QC bridge" on the desktop
    python -m qc_agent uninstall-shortcut   remove it

No secret is ever accepted on the command line; everything comes from agent.env.
"""
from __future__ import annotations

import argparse
import logging
import logging.handlers
import sys
import threading
from pathlib import Path

from . import AGENT_VERSION

EXIT_CONFIG, EXIT_RUNNING = 2, 4
log = logging.getLogger("qc_agent")


def _console_utf8():
    for s in (sys.stdout, sys.stderr):
        try:
            s.reconfigure(encoding="utf-8", errors="replace")
        except Exception:  # noqa: BLE001
            pass


def setup_logging(home: Path):
    log.setLevel(logging.INFO)
    log.propagate = False
    con = logging.StreamHandler(sys.stdout)
    con.setFormatter(logging.Formatter("%(asctime)s  %(message)s", "%H:%M:%S"))
    log.addHandler(con)
    logs = home / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    fh = logging.handlers.RotatingFileHandler(logs / "agent.log", maxBytes=1_000_000, backupCount=5,
                                              encoding="utf-8")
    fh.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
    log.addHandler(fh)


def _pause(msg: str, seconds: float | None = None):
    print(msg, flush=True)
    if seconds is not None:
        threading.Event().wait(seconds)
        return
    try:
        input("Press Enter to close this window...")
    except (EOFError, KeyboardInterrupt):
        pass


def _install_console_handler(agent):
    """Closing the console window (or Ctrl+C / Ctrl+Break / logoff) asks the agent to stop and
    gives it a few seconds to write the "exiting" heartbeat. Windows kills the process ~5 s after
    a close event regardless."""
    if sys.platform != "win32":
        return None
    import ctypes
    from ctypes import wintypes

    done = threading.Event()

    @ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.DWORD)
    def handler(event):
        agent.request_exit("the window was closed" if event == 2 else "stop requested")
        done.wait(4.0)
        return True

    ctypes.windll.kernel32.SetConsoleCtrlHandler(handler, True)
    return handler, done


def cmd_run(args) -> int:
    from . import config as cfg
    from .catalog import Catalog, CatalogError
    from .device import DeviceError
    from .lock import AlreadyRunning, InstanceLock

    home = Path(args.home) if args.home else cfg.default_home()
    try:
        config = cfg.load(Path(args.config) if args.config else home / "agent.env")
    except cfg.ConfigError as e:
        _pause(f"QC bridge: configuration problem: {e}")
        return EXIT_CONFIG
    lock = InstanceLock(home / "agent.lock")
    try:
        lock.acquire()
    except AlreadyRunning as e:
        _pause(f"QC bridge: {e}. Use that window, or close it first.", seconds=8)
        return EXIT_RUNNING
    try:
        setup_logging(home)
        log.info("QC bridge %s starting", AGENT_VERSION)
        try:
            catalog = Catalog.load()
            from .qcmcp_backend import QcMcpBackend
            device = QcMcpBackend()
        except (CatalogError, DeviceError) as e:
            log.error("cannot start: %s", e)
            _pause("")
            return EXIT_CONFIG
        from .agent import Agent
        from .ledger import LedgerError
        from .s3 import S3Store
        store = S3Store(config.endpoint, config.bucket, config.access_key_id, config.secret_access_key)
        try:
            agent = Agent(config, store, device, catalog, home)
        except LedgerError as e:
            log.error("cannot start: %s", e)
            _pause("")
            return EXIT_CONFIG
        hook = _install_console_handler(agent)
        try:
            code = agent.run()
        except KeyboardInterrupt:
            agent.request_exit("stop requested")
            code = 0
        finally:
            if hook:
                hook[1].set()
        _pause("This window closes in 10 seconds.", seconds=10)
        return code
    finally:
        lock.release()


def cmd_check(args) -> int:
    from . import config as cfg
    from .catalog import Catalog, CatalogError
    from .vendor_pins import COMMIT, mismatches

    ok = True
    home = Path(args.home) if args.home else cfg.default_home()
    try:
        c = cfg.load(Path(args.config) if args.config else home / "agent.env")
        print(f"config: OK  bucket={c.bucket} slots={','.join(c.allowed_slots.labels())} "
              f"firmware={c.expected_firmware} idle={c.idle_minutes} min key id={c.current_kid}"
              + (" (+ NEXT key)" if len(c.keys) > 1 else ""))
    except cfg.ConfigError as e:
        ok = False
        print(f"config: FAILED  {e}")
    bad = mismatches()
    print(f"vendored qc-mcp {COMMIT[:7]}: " + ("OK" if not bad else "FAILED  " + "; ".join(bad)))
    ok &= not bad
    try:
        cat = Catalog.load()
        print(f"catalog: OK  {len(cat.models)} models, sha256 {cat.sha256[:12]}...")
    except CatalogError as e:
        ok = False
        print(f"catalog: FAILED  {e}")
    if not bad:
        try:
            from .qcmcp_backend import load_vendored
            mods = load_vendored()
            import google.protobuf
            mods["protocol"].message_class("Grid")
            print(f"transport import: OK  (protobuf {google.protobuf.__version__})")
        except Exception as e:  # noqa: BLE001
            ok = False
            print(f"transport import: FAILED  {type(e).__name__}: {e}")
    print("all checks passed" if ok else "some checks FAILED")
    return 0 if ok else 1


def cmd_install(args) -> int:
    from .shortcut import install
    try:
        path = install()
    except (RuntimeError, OSError, ValueError) as e:
        print(f"install-shortcut: {e}")
        return 1
    print(f"Created {path}")
    return 0


def cmd_uninstall(args) -> int:
    from .shortcut import uninstall
    try:
        path = uninstall()
    except (RuntimeError, OSError) as e:
        print(f"uninstall-shortcut: {e}")
        return 1
    print(f"Removed {path}" if path else "No shortcut to remove.")
    return 0


def main(argv=None) -> int:
    _console_utf8()
    ap = argparse.ArgumentParser(prog="python -m qc_agent", description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name in ("run", "check"):
        p = sub.add_parser(name)
        p.add_argument("--home", help=argparse.SUPPRESS)    # tests only
        p.add_argument("--config", help=argparse.SUPPRESS)  # tests only
    sub.add_parser("install-shortcut")
    sub.add_parser("uninstall-shortcut")
    args = ap.parse_args(argv)
    return {"run": cmd_run, "check": cmd_check, "install-shortcut": cmd_install,
            "uninstall-shortcut": cmd_uninstall}[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
