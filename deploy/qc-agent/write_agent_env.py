"""Write the QC bridge's config file, %LOCALAPPDATA%\\qc-agent\\agent.env.

Run it on the Windows PC that the Quad Cortex is plugged into:

    py -3 deploy\\qc-agent\\write_agent_env.py

It asks for each value (secrets are typed hidden and never echoed), writes the
file, restricts it to your Windows user with icacls, and can test the R2
credentials with a read-only listing of the jobs bucket. Standard library
only; nothing is sent anywhere except that one test request to your own R2
endpoint. This script contains no secrets and is safe to keep.

The variable names are the agent's configuration contract: the home agent
(plan P4, deploy/qc-agent/) reads this file with exactly these names. See
docs/design/qc-device-bridge-plan.md section 5.
"""

import datetime
import getpass
import hashlib
import hmac
import os
import re
import secrets
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_BUCKET = "sound-profile-builder-device-jobs"
DEFAULT_SLOTS = "1A-32H"  # owner decision: creation allowed in every bank (empty slots only)
DEFAULT_FIRMWARE = "4.1.0"
DEFAULT_IDLE_MINUTES = "30"

KEY_RE = re.compile(r"[0-9a-fA-F]{64}")
SLOT = r"(?:[1-9]|[12][0-9]|3[0-2])[A-H]"
SLOTS_RE = re.compile(rf"{SLOT}(?:-{SLOT})?(?:,{SLOT}(?:-{SLOT})?)*")
BUCKET_RE = re.compile(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]")


def ask(prompt, default=None, secret=False, check=None, error="invalid value"):
    shown = f" [{default}]" if default else ""
    while True:
        raw = getpass.getpass(f"{prompt}{shown}: ") if secret else input(f"{prompt}{shown}: ")
        value = raw.strip() or (default or "")
        if not value:
            print("  a value is required")
            continue
        if any(c.isspace() for c in value) or any(ord(c) < 32 for c in value):
            print("  no spaces or control characters, please")
            continue
        if check and not check(value):
            print(f"  {error}")
            continue
        return value


def mask(value):
    return value[:4] + "..." + value[-2:] if len(value) > 8 else "***"


# ---- read-only R2 check (AWS Signature V4, stdlib only) ----

def _sign(key, msg):
    return hmac.new(key, msg.encode("utf-8"), hashlib.sha256).digest()


def check_r2(endpoint, bucket, access_key, secret_key):
    """GET /<bucket>?list-type=2&max-keys=1, signed with SigV4 (region auto)."""
    host = urllib.parse.urlparse(endpoint).netloc
    now = datetime.datetime.now(datetime.timezone.utc)
    amz_date, day = now.strftime("%Y%m%dT%H%M%SZ"), now.strftime("%Y%m%d")
    query = "list-type=2&max-keys=1"
    payload_hash = hashlib.sha256(b"").hexdigest()
    canonical = "\n".join([
        "GET", f"/{bucket}", query,
        f"host:{host}\nx-amz-content-sha256:{payload_hash}\nx-amz-date:{amz_date}\n",
        "host;x-amz-content-sha256;x-amz-date", payload_hash,
    ])
    scope = f"{day}/auto/s3/aws4_request"
    to_sign = "\n".join(["AWS4-HMAC-SHA256", amz_date, scope, hashlib.sha256(canonical.encode()).hexdigest()])
    k = _sign(_sign(_sign(_sign(("AWS4" + secret_key).encode(), day), "auto"), "s3"), "aws4_request")
    signature = hmac.new(k, to_sign.encode(), hashlib.sha256).hexdigest()
    req = urllib.request.Request(f"{endpoint}/{bucket}?{query}", headers={
        "x-amz-date": amz_date, "x-amz-content-sha256": payload_hash,
        "Authorization": f"AWS4-HMAC-SHA256 Credential={access_key}/{scope}, "
                         f"SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature={signature}",
    })
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.status == 200, f"HTTP {resp.status}"
    except urllib.error.HTTPError as e:
        code = re.search(rb"<Code>([^<]+)</Code>", e.read() or b"")
        return False, f"HTTP {e.code} {code.group(1).decode() if code else ''}".strip()
    except OSError as e:
        return False, f"network error: {e}"


# ---- file writing ----

def restrict_to_current_user(path):
    """Remove inherited permissions and grant only the current user (Windows)."""
    if os.name != "nt":
        os.chmod(path, 0o600)
        return True, "chmod 600"
    user = os.environ.get("USERNAME", "")
    domain = os.environ.get("USERDOMAIN", "")
    who = f"{domain}\\{user}" if domain else user
    if not user:
        return False, "USERNAME is not set"
    result = subprocess.run(["icacls", path, "/inheritance:r", "/grant:r", f"{who}:(F)"],
                            capture_output=True, text=True)
    if result.returncode != 0:
        return False, (result.stderr or result.stdout).strip()
    return True, f"only {who} can read it"


def main():
    base = os.environ.get("LOCALAPPDATA")
    if not base:
        sys.exit("LOCALAPPDATA is not set; run this on the Windows PC as your normal user.")
    folder = os.path.join(base, "qc-agent")
    path = os.path.join(folder, "agent.env")

    print(f"This writes {path}\n")
    if os.path.exists(path):
        if input("agent.env already exists. Overwrite it? [y/N]: ").strip().lower() != "y":
            sys.exit("Nothing changed.")

    endpoint = ask("R2 endpoint (https://<account-id>.r2.cloudflarestorage.com)",
                   check=lambda v: v.startswith("https://") and "/" not in v[len("https://"):],
                   error="must look like https://<account-id>.r2.cloudflarestorage.com").rstrip("/")
    bucket = ask("Jobs bucket", DEFAULT_BUCKET, check=BUCKET_RE.fullmatch, error="not a valid bucket name")
    access_key = ask("Bridge token Access Key ID")
    secret_key = ask("Bridge token Secret Access Key (hidden)", secret=True)

    print("\nDEVICE_JOB_KEY must be the same value as the Fly secret.")
    job_key = getpass.getpass("DEVICE_JOB_KEY (hidden; leave empty to generate a new one): ").strip()
    generated = False
    if not job_key:
        job_key, generated = secrets.token_hex(32), True
    while not KEY_RE.fullmatch(job_key):
        job_key = getpass.getpass("  must be exactly 64 hex characters, try again (hidden): ").strip()

    slots = ask("Slots the bridge may create presets in (1A-32H = all; or e.g. 20A-25H,30A)", DEFAULT_SLOTS,
                check=lambda v: bool(SLOTS_RE.fullmatch(v.upper())), error="use ranges like 20A-25H").upper()
    firmware = ask("Expected Quad Cortex firmware", DEFAULT_FIRMWARE,
                   check=lambda v: bool(re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", v)),
                   error="use the exact three-part version the unit reports, e.g. 4.1.0")
    idle = ask("Idle minutes before the bridge exits", DEFAULT_IDLE_MINUTES,
               check=lambda v: v.isdigit() and 1 <= int(v) <= 240, error="a number of minutes, 1-240")

    print("\nChecking the bridge token against R2 (read-only list of the jobs bucket)...")
    ok, detail = check_r2(endpoint, bucket, access_key, secret_key)
    print(f"  {'OK' if ok else 'FAILED'}: {detail}")
    if not ok and input("Write the file anyway? [y/N]: ").strip().lower() != "y":
        sys.exit("Nothing written.")

    lines = [
        "# QC bridge config. Secrets: keep this file private; never commit or paste it.",
        f"QC_AGENT_R2_ENDPOINT={endpoint}",
        f"QC_AGENT_R2_BUCKET={bucket}",
        f"QC_AGENT_R2_ACCESS_KEY_ID={access_key}",
        f"QC_AGENT_R2_SECRET_ACCESS_KEY={secret_key}",
        f"DEVICE_JOB_KEY={job_key.lower()}",
        "# DEVICE_JOB_KEY_NEXT=   (only during a key rotation)",
        f"QC_AGENT_ALLOWED_SLOTS={slots}",
        f"QC_AGENT_EXPECTED_FIRMWARE={firmware}",
        f"QC_AGENT_IDLE_MINUTES={idle}",
        "",
    ]
    os.makedirs(folder, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=folder, prefix=".agent.env.")
    with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(lines))
    restricted, how = restrict_to_current_user(tmp)
    os.replace(tmp, path)

    print(f"\nWrote {path}")
    print(f"  access key {mask(access_key)}, secret and DEVICE_JOB_KEY hidden")
    print(f"  permissions: {how}" if restricted else f"  WARNING: could not restrict permissions: {how}")
    if generated:
        print("\nA new DEVICE_JOB_KEY was generated. Set the same value on Fly (run where fly is logged in):")
        print(f"  fly secrets set DEVICE_JOB_KEY={job_key.lower()}")
        print("Then close this window so the key isn't left on screen.")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit("\nCancelled; nothing written.")
