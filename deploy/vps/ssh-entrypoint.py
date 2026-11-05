#!/usr/bin/env python3
"""Restricted SSH transport for the dedicated GitHub Actions key."""
import hashlib
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

MAX_UPLOAD = 3 * 1024**3


def parse_command(command):
    match = re.fullmatch(r"upload ([0-9a-f]{40}) ([0-9a-f]{64})", command)
    if match:
        return "upload", *match.groups()
    match = re.fullmatch(r"deploy ([0-9a-f]{40})", command)
    if match:
        return "deploy", match.group(1), None
    raise ValueError("Only a release upload or deployment is allowed.")


def upload(root, revision, expected, source):
    directory = root / "incoming"
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=revision + ".", dir=directory)
    temporary = Path(temporary)
    try:
        digest, size = hashlib.sha256(), 0
        with os.fdopen(descriptor, "wb") as output:
            while chunk := source.read(1024 * 1024):
                size += len(chunk)
                if size > MAX_UPLOAD:
                    raise ValueError("Release exceeds the upload size limit.")
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest() != expected:
            raise ValueError("Release upload checksum does not match.")
        temporary.replace(directory / (revision + ".tar"))
    finally:
        temporary.unlink(missing_ok=True)


def run_deployment(root, revision):
    # Keep the server-side operation alive if the runner's SSH channel closes.
    logs = root / "ci/logs"
    logs.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor, filename = tempfile.mkstemp(prefix=revision + "-", suffix=".log", dir=logs)
    with os.fdopen(descriptor, "w+b") as log:
        process = subprocess.Popen(
            [sys.executable, str(root / "ci/deploy-release.py"), revision],
            stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT,
            start_new_session=True)
        status = process.wait()
        log.seek(0)
        for chunk in iter(lambda: log.read(65536), b""):
            sys.stdout.write(chunk.decode(errors="replace"))
    for old in sorted(logs.glob("*.log"), key=lambda p: p.stat().st_mtime, reverse=True)[14:]:
        old.unlink(missing_ok=True)
    return status


def main():
    root = Path("/opt/kg2700")
    operation, revision, digest = parse_command(os.environ.get("SSH_ORIGINAL_COMMAND", ""))
    if operation == "upload":
        upload(root, revision, digest, sys.stdin.buffer)
        print("Release upload verified.")
    else:
        return run_deployment(root, revision)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        sys.exit(str(error))
