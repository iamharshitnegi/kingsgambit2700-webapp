#!/usr/bin/env python3
"""Deploy prebuilt images on the single VPS without replacing private settings."""
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

SERVICES = ("api", "worker", "realtime", "frontend", "river-migrate", "bootstrap-admin")
APPLICATION = ("api", "worker", "realtime", "frontend", "caddy")
SHA = re.compile(r"[0-9a-f]{40}")


def extract_source(archive, destination):
    with tarfile.open(archive, "r:gz") as source:
        members = source.getmembers()
        if sum(member.size for member in members) > 256 * 1024**2:
            raise ValueError("Unpacked source exceeds 256 MiB.")
        for member in members:
            path = PurePosixPath(member.name)
            if (path.is_absolute() or ".." in path.parts or
                    not (member.isfile() or member.isdir()) or
                    ".git" in path.parts or
                    (path.name.startswith(".env") and path.name != ".env.example") or
                    member.name == "deploy/vps/.release"):
                raise ValueError("Unsafe or private path in release source.")
        source.extractall(destination, members=members, filter="data")


class Deployer:
    def __init__(self, root):
        self.root = Path(root)
        self.environment = self.root / "shared/production.env"
        self.secrets = []
        if self.environment.is_file():
            for line in self.environment.read_text().splitlines():
                key, separator, value = line.partition("=")
                if separator and any(word in key for word in ("PASSWORD", "SECRET", "KEY")):
                    value = value.strip().strip("\"'")
                    if len(value) >= 8:
                        self.secrets.append(value)

    def redact(self, message):
        for value in self.secrets:
            message = message.replace(value, "[redacted]")
        return message

    def execute(self, command, *, quiet=False, stdout=None):
        result = subprocess.run(command, stdout=subprocess.PIPE if stdout is None else stdout,
                                stderr=subprocess.PIPE, text=stdout is None, check=False)
        output = result.stdout or ""
        error = result.stderr or ""
        if isinstance(error, bytes):
            error = error.decode(errors="replace")
        if not quiet and output:
            print(self.redact(output), end="", flush=True)
        if error:
            print(self.redact(error), end="", file=sys.stderr, flush=True)
        if result.returncode:
            raise RuntimeError("Deployment command failed: " + command[0])
        return output

    def compose(self, release, *arguments, **options):
        return self.execute(["bash", str(release / "deploy/vps/compose.sh"), *arguments], **options)

    def unpack(self, bundle, directory, revision):
        with tarfile.open(bundle) as archive:
            members = archive.getmembers()
            if (len(members) != 3 or {item.name for item in members} !=
                    {"manifest.json", "source.tar.gz", "images.tar.gz"} or
                    any(not item.isfile() for item in members) or
                    sum(item.size for item in members) > 3 * 1024**3):
                raise ValueError("Invalid release bundle contents.")
            archive.extractall(directory, filter="data")
        manifest = json.loads((directory / "manifest.json").read_text())
        if manifest.get("version") != 1 or manifest.get("revision") != revision:
            raise ValueError("Release manifest does not match the requested commit.")
        for name in ("source.tar.gz", "images.tar.gz"):
            with (directory / name).open("rb") as source:
                digest = hashlib.file_digest(source, "sha256").hexdigest()
            if digest != manifest.get(name):
                raise ValueError("Release file checksum does not match: " + name)
        return manifest

    def prepare(self, directory, manifest, revision):
        releases = self.root / "releases"
        releases.mkdir(mode=0o700, exist_ok=True)
        candidate = releases / revision
        if candidate.exists():
            previous = json.loads((candidate / ".deployment.json").read_text())
            identity = ("version", "revision", "source.tar.gz", "web_domain", "api_domain")
            if any(previous.get(key) != manifest.get(key) for key in identity):
                raise ValueError("Existing release has different contents.")
            return candidate
        with tempfile.TemporaryDirectory(prefix=".prepare-", dir=releases) as temporary:
            stage = Path(temporary) / "source"
            stage.mkdir()
            extract_source(directory / "source.tar.gz", stage)
            for name in ("deploy/vps/compose.sh", "deploy/vps/compose.yaml", "deploy/vps/Caddyfile"):
                if not (stage / name).is_file():
                    raise ValueError("Release is missing deployment files.")
            (stage / "deploy/vps/.env").symlink_to(self.environment)
            (stage / "deploy/vps/.release").write_text(revision + "\n")
            (stage / ".deployment.json").write_text(json.dumps(manifest) + "\n")
            stage.rename(candidate)
        return candidate

    def backup(self, candidate):
        directory = self.root / "backups"
        directory.mkdir(mode=0o700, exist_ok=True)
        name = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S.%fZ") + ".dump"
        backup = directory / name
        try:
            descriptor = os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(descriptor, "wb") as output:
                self.compose(candidate, "exec", "-T", "postgres", "pg_dump", "-U", "kg2700",
                             "-d", "kg2700", "-Fc", stdout=output)
            if backup.stat().st_size == 0:
                raise RuntimeError("Database backup is empty.")
            with backup.open("rb") as source:
                result = subprocess.run(
                    ["bash", str(candidate / "deploy/vps/compose.sh"), "exec", "-T", "postgres",
                     "pg_restore", "--list"], stdin=source, stdout=subprocess.DEVNULL,
                    stderr=subprocess.PIPE, check=False)
            if result.returncode:
                raise RuntimeError("Database backup could not be validated.")
        except Exception:
            backup.unlink(missing_ok=True)
            raise
        print("Pre-deployment PostgreSQL backup validated.", flush=True)

    def health(self, candidate, manifest):
        ready = json.loads(self.compose(candidate, "exec", "-T", "api", "wget", "-qO-",
                                        "http://127.0.0.1:8080/health/ready", quiet=True))
        if ready.get("status") != "ok":
            raise RuntimeError("API readiness failed.")
        for url in ("https://" + manifest["web_domain"] + "/sign-in",
                    "https://" + manifest["api_domain"] + "/health/ready"):
            for attempt in range(12):
                try:
                    with urllib.request.urlopen(url, timeout=10) as response:
                        if (response.status != 200 or
                                response.headers.get("X-KG-Release") != manifest["revision"]):
                            raise RuntimeError("Public HTTPS did not reach this release.")
                    break
                except (OSError, RuntimeError):
                    if attempt == 11:
                        raise RuntimeError("Public HTTPS check failed: " + url) from None
                    time.sleep(5)
        print("Internal readiness and public HTTPS checks passed.", flush=True)

    def activate(self, candidate):
        current = self.root / "current"
        if current.exists() and not current.is_symlink():
            raise RuntimeError("The current release must be a symlink.")
        if current.is_symlink() and current.resolve() != candidate:
            self.link(self.root / "previous", current.resolve())
        self.link(current, candidate)

    @staticmethod
    def link(path, target):
        temporary = path.with_name(path.name + ".new")
        temporary.symlink_to(target)
        try:
            temporary.replace(path)
        finally:
            temporary.unlink(missing_ok=True)

    def prune(self):
        protected = {p.resolve() for p in (self.root / "current", self.root / "previous") if p.exists()}
        releases = sorted((p for p in (self.root / "releases").iterdir()
                           if p.is_dir() and SHA.fullmatch(p.name)),
                          key=lambda p: p.stat().st_mtime, reverse=True)
        keep = set(protected)
        for release in releases:
            if len(keep & set(releases)) < 3:
                keep.add(release)
        for release in releases:
            if release not in keep:
                for service in SERVICES:
                    # Docker refuses removal if a container still needs an image.
                    subprocess.run(["docker", "image", "rm", "kg2700-" + service + ":" + release.name],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
                shutil.rmtree(release)
        backups = sorted((self.root / "backups").glob("*.dump"), reverse=True)
        for backup in backups[7:]:
            backup.unlink()

    def deploy(self, revision):
        if not SHA.fullmatch(revision):
            raise ValueError("Expected a full commit SHA.")
        if not self.environment.is_file():
            raise RuntimeError("Run install-actions.sh on the VPS first.")
        if shutil.disk_usage(self.root).free < 2 * 1024**3:
            raise RuntimeError("At least 2 GiB of free disk space is required.")
        previous = (self.root / "current").resolve() if (self.root / "current").exists() else None
        candidate, rollout = None, False
        with tempfile.TemporaryDirectory(prefix=".deploy-", dir=self.root / "incoming") as temporary:
            directory = Path(temporary)
            bundle = directory / "release.tar"
            (self.root / "incoming" / (revision + ".tar")).replace(bundle)
            try:
                manifest = self.unpack(bundle, directory, revision)
                candidate = self.prepare(directory, manifest, revision)
                config = json.loads(self.compose(candidate, "config", "--format", "json", quiet=True))
                web = config["services"]["caddy"]["environment"]["WEB_DOMAIN"]
                api = config["services"]["caddy"]["environment"]["API_DOMAIN"]
                if web != manifest["web_domain"] or api != manifest["api_domain"]:
                    raise ValueError("Image build domains do not match the VPS environment.")
                if candidate == previous:
                    # A CI rerun can rebuild byte-different images for the same
                    # source. Keep active image tags stable so rollback remains possible.
                    active = json.loads((candidate / ".deployment.json").read_text())
                    manifest["images.tar.gz"] = active["images.tar.gz"]
                    print("Retaining image artifacts for the already-active commit.", flush=True)
                else:
                    self.execute(["docker", "load", "--input", str(directory / "images.tar.gz")])
                for service in SERVICES:
                    label = self.execute(["docker", "image", "inspect", "--format",
                                          '{{index .Config.Labels "org.opencontainers.image.revision"}}',
                                          "kg2700-" + service + ":" + revision], quiet=True).strip()
                    if label != revision:
                        raise ValueError("Image revision label does not match the release.")
                self.compose(candidate, "up", "-d", "--no-recreate", "--wait", "--wait-timeout", "180",
                             "postgres", "redis", "storage", "clamav")
                self.backup(candidate)
                self.compose(candidate, "--profile", "tools", "run", "--rm", "--no-deps", "migrate")
                self.compose(candidate, "--profile", "tools", "run", "--rm", "--no-deps", "river-migrate", "up")
                self.compose(candidate, "--profile", "tools", "run", "--rm", "--no-deps", "river-migrate", "validate")
                rollout = True
                self.compose(candidate, "up", "-d", "--no-deps", "--no-build", "--pull", "never", "--wait",
                             "--wait-timeout", "180", *APPLICATION)
                self.health(candidate, manifest)
                metadata = candidate / ".deployment.json.new"
                metadata.write_text(json.dumps(manifest) + "\n")
                metadata.replace(candidate / ".deployment.json")
                self.activate(candidate)
            except Exception:
                if rollout and previous is not None:
                    try:
                        self.compose(previous, "up", "-d", "--no-deps", "--no-build", "--pull", "never", "--wait",
                                     "--wait-timeout", "180", *APPLICATION)
                        print("Previous application images restored; database migrations retained.", flush=True)
                    except Exception as error:
                        print(self.redact("Automatic application rollback failed: " + str(error)), file=sys.stderr)
                raise
        try:
            self.prune()
        except OSError as error:
            print(self.redact("Deployment succeeded; retention cleanup needs attention: " + str(error)), file=sys.stderr)
        print("Production release active: " + revision, flush=True)


def main():
    os.umask(0o077)
    root = Path("/opt/kg2700")
    deployer = Deployer(root)
    with (root / ".deployment.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        try:
            deployer.deploy(sys.argv[1] if len(sys.argv) == 2 else "")
        except Exception as error:
            sys.exit(deployer.redact(str(error)))


if __name__ == "__main__":
    main()
