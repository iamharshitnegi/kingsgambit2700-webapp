"""Exercise deployment failure paths without a VPS, Docker daemon, or real data."""
import contextlib
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import runpy
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

REPO = Path(__file__).resolve().parents[2]


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, REPO / "deploy/vps" / filename)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


deploy = module("deploy_release", "deploy-release.py")
transport = module("ssh_entrypoint", "ssh-entrypoint.py")
REVISION = "a" * 40


def make_tar(path, files, mode="w:gz"):
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w") as archive:
        for name, value in files.items():
            data = value.encode() if isinstance(value, str) else value
            member = tarfile.TarInfo(name)
            member.size = len(data)
            archive.addfile(member, io.BytesIO(data))
    contents = buffer.getvalue()
    path.write_bytes(gzip.compress(contents, mtime=0) if mode == "w:gz" else contents)


class FakeDeployer(deploy.Deployer):
    def __init__(self, root, failure=None):
        super().__init__(root)
        self.commands, self.failure = [], failure

    def execute(self, command, *, quiet=False, stdout=None):
        self.commands.append(command)
        if command[0] == "docker":
            return REVISION if "inspect" in command else ""
        if self.failure == "migration" and command[-1] == "migrate":
            raise RuntimeError("migration failed")
        if self.failure == "backup" and "pg_dump" in command:
            raise RuntimeError("backup failed")
        if "pg_dump" in command:
            stdout.write(b"PGDMP-test-fixture")
        if "config" in command:
            return json.dumps({"services": {"caddy": {"environment": {
                "WEB_DOMAIN": "example.test", "API_DOMAIN": "api.example.test"}}}})
        return ""

    def health(self, candidate, manifest):
        self.commands.append(["health"])
        if self.failure == "health":
            raise RuntimeError("health failed")


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        for name in ["shared", "incoming", "app/deploy/vps"]:
            (self.root / name).mkdir(parents=True)
        self.environment = self.root / "shared/production.env"
        self.environment.write_text("KG_SMTP_PASSWORD='private-test-password'\nRELEASE=initial\n")
        (self.root / "app/deploy/vps/compose.sh").write_text("legacy")
        (self.root / "current").symlink_to(self.root / "app")

    def bundle(self, extra_source=None, images=b"fake docker archive"):
        source = self.root / "source.tar.gz"
        files = {"deploy/vps/compose.sh": "wrapper", "deploy/vps/compose.yaml": "name: kg2700-production",
                 "deploy/vps/Caddyfile": "config"}
        files.update(extra_source or {})
        make_tar(source, files)
        manifest = {"version": 1, "revision": REVISION, "web_domain": "example.test",
                    "api_domain": "api.example.test", "source.tar.gz": hashlib.sha256(source.read_bytes()).hexdigest(),
                    "images.tar.gz": hashlib.sha256(images).hexdigest()}
        make_tar(self.root / "incoming" / (REVISION + ".tar"), {
            "manifest.json": json.dumps(manifest), "source.tar.gz": source.read_bytes(),
            "images.tar.gz": images}, mode="w")

    def run_deployment(self, failure=None):
        instance = FakeDeployer(self.root, failure)
        with patch.object(deploy.subprocess, "run", return_value=SimpleNamespace(returncode=0)), \
                contextlib.redirect_stdout(io.StringIO()):
            instance.deploy(REVISION)
        return instance

    def test_success_preserves_secrets_and_backs_up_before_migration(self):
        self.bundle()
        original = self.environment.read_bytes()
        instance = self.run_deployment()
        current = self.root / "releases" / REVISION
        self.assertEqual((self.root / "current").resolve(), current)
        self.assertEqual((self.root / "previous").resolve(), self.root / "app")
        self.assertEqual((current / "deploy/vps/.env").resolve(), self.environment)
        self.assertEqual(self.environment.read_bytes(), original)
        self.assertEqual((current / "deploy/vps/.release").read_text().strip(), REVISION)
        self.assertTrue(list((self.root / "backups").glob("*.dump")))
        for backup_file in (self.root / "backups").glob("*.dump"):
            self.assertEqual(backup_file.stat().st_mode & 0o777, 0o600)
        backup = next(i for i, command in enumerate(instance.commands) if "pg_dump" in command)
        migrate = next(i for i, command in enumerate(instance.commands) if command[-1] == "migrate")
        rollout = next(i for i, command in enumerate(instance.commands) if "--no-build" in command)
        self.assertLess(backup, migrate)
        self.assertLess(migrate, rollout)
        self.assertFalse((self.root / "incoming" / (REVISION + ".tar")).exists())

    def test_backup_and_migration_failures_do_not_replace_running_application(self):
        for failure in ["backup", "migration"]:
            with self.subTest(failure=failure):
                self.bundle()
                instance = FakeDeployer(self.root, failure)
                with patch.object(deploy.subprocess, "run", return_value=SimpleNamespace(returncode=0)), \
                        contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
                    instance.deploy(REVISION)
                self.assertEqual((self.root / "current").resolve(), self.root / "app")
                self.assertFalse(any("--no-build" in command for command in instance.commands))

    def test_health_failure_restores_previous_images_without_down_migrations(self):
        self.bundle()
        instance = FakeDeployer(self.root, "health")
        with patch.object(deploy.subprocess, "run", return_value=SimpleNamespace(returncode=0)), \
                contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
            instance.deploy(REVISION)
        rollouts = [command for command in instance.commands if "--no-build" in command]
        self.assertEqual(len(rollouts), 2)
        self.assertIn(str(self.root / "app/deploy/vps/compose.sh"), rollouts[-1])
        self.assertEqual((self.root / "current").resolve(), self.root / "app")
        self.assertFalse(any("down" in command for command in instance.commands))

    def test_failed_commit_can_be_retried_with_rebuilt_images(self):
        self.bundle()
        with self.assertRaises(RuntimeError):
            self.run_deployment("health")
        self.bundle(images=b"rebuilt images")
        self.run_deployment()
        metadata = json.loads((self.root / "current/.deployment.json").read_text())
        self.assertEqual(metadata["images.tar.gz"], hashlib.sha256(b"rebuilt images").hexdigest())

    def test_redeploying_active_commit_keeps_its_existing_image_artifacts(self):
        self.bundle()
        self.run_deployment()
        original = (self.root / "current/.deployment.json").read_bytes()
        self.bundle(images=b"rebuilt images")
        instance = self.run_deployment()
        self.assertFalse(any(command[:2] == ["docker", "load"] for command in instance.commands))
        self.assertEqual((self.root / "current/.deployment.json").read_bytes(), original)

    def test_source_archive_cannot_replace_private_environment_or_escape_release(self):
        for name in ["../../shared/production.env", "/etc/cron.d/test", "deploy/vps/.env"]:
            with self.subTest(path=name):
                self.bundle({name: "bad"})
                with self.assertRaises(ValueError):
                    self.run_deployment()
                self.assertIn("private-test-password", self.environment.read_text())

    def test_release_checksum_is_verified_before_any_docker_command(self):
        self.bundle()
        path = self.root / "incoming" / (REVISION + ".tar")
        with tarfile.open(path) as archive:
            files = {member.name: archive.extractfile(member).read() for member in archive.getmembers()}
        files["images.tar.gz"] = b"tampered"
        make_tar(path, files, mode="w")
        instance = FakeDeployer(self.root)
        with self.assertRaises(ValueError):
            instance.deploy(REVISION)
        self.assertEqual(instance.commands, [])

    def test_transport_rejects_shell_commands_and_checksum_mismatch(self):
        for command in ["bash", "deploy main", "deploy " + REVISION + ";id", "upload ../bad x"]:
            with self.subTest(command=command), self.assertRaises(ValueError):
                transport.parse_command(command)
        self.assertEqual(transport.parse_command("deploy " + REVISION), ("deploy", REVISION, None))
        content = b"release bundle"
        digest = hashlib.sha256(content).hexdigest()
        transport.upload(self.root, REVISION, digest, io.BytesIO(content))
        with self.assertRaises(ValueError):
            transport.upload(self.root, REVISION, "0" * 64, io.BytesIO(b"bad"))
        self.assertEqual((self.root / "incoming" / (REVISION + ".tar")).read_bytes(), content)

    def test_server_operation_has_independent_session_and_private_log(self):
        helper = self.root / "ci/deploy-release.py"
        helper.parent.mkdir()
        helper.write_text("print('deployment finished')\nraise SystemExit(7)\n")
        output = io.StringIO()
        with patch.object(transport.subprocess, "Popen", wraps=subprocess.Popen) as spawn, \
                contextlib.redirect_stdout(output):
            result = transport.run_deployment(self.root, REVISION)
        self.assertEqual(result, 7)
        self.assertTrue(spawn.call_args.kwargs["start_new_session"])
        self.assertIn("deployment finished", output.getvalue())
        log = next((self.root / "ci/logs").glob("*.log"))
        self.assertEqual(log.stat().st_mode & 0o777, 0o600)

    def test_compose_wrapper_uses_release_tag_without_sourcing_private_environment(self):
        directory = self.root / "wrapper"
        directory.mkdir()
        (directory / "compose.sh").write_bytes((REPO / "deploy/vps/compose.sh").read_bytes())
        (directory / ".release").write_text(REVISION + "\n")
        (directory / ".env").write_text("RELEASE=initial\nDO_NOT_EXECUTE=$(exit 99)\n")
        docker = directory / "docker"
        docker.write_text('#!/bin/sh\nprintf "%s" "$RELEASE"\n')
        docker.chmod(0o700)
        env = dict(os.environ, PATH=str(directory) + ":" + os.environ["PATH"])
        env.pop("RELEASE", None)
        result = subprocess.run(["bash", str(directory / "compose.sh"), "config"],
                                env=env, text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout, REVISION)

    def test_google_helper_updates_shared_environment_without_replacing_symlink(self):
        directory = self.root / "google"
        directory.mkdir()
        helper = directory / "configure-google.py"
        helper.write_bytes((REPO / "deploy/vps/configure-google.py").read_bytes())
        environment_link = directory / ".env"
        environment_link.symlink_to(self.environment)
        output = io.StringIO()
        with patch("sys.stdin", SimpleNamespace(isatty=lambda: True)), \
                patch("builtins.input", side_effect=["sender@gmail.com", "owner@gmail.com", "123.apps.googleusercontent.com"]), \
                patch("getpass.getpass", return_value="a" * 16), contextlib.redirect_stdout(output):
            runpy.run_path(str(helper))
        self.assertTrue(environment_link.is_symlink())
        self.assertIn("KG_GOOGLE_CLIENT_ID='123.apps.googleusercontent.com'", self.environment.read_text())
        self.assertIn("RELEASE=initial", self.environment.read_text())
        self.assertEqual(self.environment.stat().st_mode & 0o777, 0o600)
        self.assertNotIn("a" * 16, output.getvalue())

    def test_retention_keeps_current_previous_and_seven_backups(self):
        releases = self.root / "releases"
        releases.mkdir()
        for index in range(1, 6):
            release = releases / (str(index) * 40)
            release.mkdir()
            os.utime(release, (index, index))
        (self.root / "current").unlink()
        (self.root / "current").symlink_to(releases / ("5" * 40))
        (self.root / "previous").symlink_to(releases / ("1" * 40))
        backups = self.root / "backups"
        backups.mkdir()
        for index in range(10):
            (backups / f"20260101T0000{index:02d}.dump").write_bytes(b"fixture")
        with patch.object(deploy.subprocess, "run") as commands:
            deploy.Deployer(self.root).prune()
        self.assertEqual({path.name for path in releases.iterdir()}, {"1" * 40, "4" * 40, "5" * 40})
        self.assertEqual(len(list(backups.iterdir())), 7)
        self.assertTrue(all(call.args[0][:3] == ["docker", "image", "rm"] for call in commands.call_args_list))

    def test_builder_packages_committed_source_and_excludes_local_private_files(self):
        checkout = self.root / "checkout"
        checkout.mkdir()
        (checkout / ".gitignore").write_text(".env\n")
        (checkout / "Dockerfile").write_text("FROM scratch\n")
        (checkout / "app.txt").write_text("committed application")
        (checkout / ".env").write_text("DO_NOT_SHIP=private\n")
        git = ["git", "-c", "user.name=Deployment Test", "-c", "user.email=test@example.test",
               "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"]
        for arguments in [["init", "--quiet"], ["add", "."], ["commit", "--quiet", "-m", "fixture"]]:
            subprocess.run(git + arguments, cwd=checkout, check=True, capture_output=True)
        revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=checkout, text=True).strip()
        (checkout / "app.txt").write_text("uncommitted application")
        commands = self.root / "commands"
        commands.mkdir()
        docker = commands / "docker"
        docker.write_text('''#!/bin/sh
if [ "$1" = save ]; then printf 'test-image'; exit 0; fi
for context; do :; done
test ! -e "$context/.env" || exit 90
if [ -f "$context/app.txt" ]; then grep -qx 'committed application' "$context/app.txt" || exit 91; fi
''')
        docker.chmod(0o700)
        output = self.root / "bundle"
        env = dict(os.environ, PATH=str(commands) + ":" + os.environ["PATH"])
        subprocess.run(["bash", str(REPO / "deploy/vps/build-release.sh"), revision, str(output)],
                       cwd=checkout, env=env, check=True, capture_output=True)
        with tarfile.open(output / "release.tar") as bundle:
            self.assertEqual(set(bundle.getnames()), {"manifest.json", "source.tar.gz", "images.tar.gz"})
            manifest = json.load(bundle.extractfile("manifest.json"))
            source_bytes = bundle.extractfile("source.tar.gz").read()
            self.assertEqual(manifest["revision"], revision)
            self.assertEqual(manifest["source.tar.gz"], hashlib.sha256(source_bytes).hexdigest())
            with tarfile.open(fileobj=io.BytesIO(source_bytes), mode="r:gz") as source:
                self.assertNotIn(".env", source.getnames())
                self.assertEqual(source.extractfile("app.txt").read(), b"committed application")


if __name__ == "__main__":
    unittest.main()
