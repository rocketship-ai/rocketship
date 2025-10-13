#!/usr/bin/env python3

import argparse
import json
import os
import socket
import subprocess
import sys
import tempfile
import time
import traceback
import urllib.error
import urllib.request
from typing import Any, Dict, Optional

try:
    from playwright.sync_api import sync_playwright
except ImportError as exc:  # pragma: no cover - import failure reported to caller
    sys.stdout.write(json.dumps({"ok": False, "error": f"playwright not available: {exc}"}) + "\n")
    sys.exit(1)


def _write(payload: Dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


def _parse_bool(value: Optional[str], default: bool = True) -> bool:
    if value is None:
        return default
    normalized = value.strip().lower()
    if normalized in {"true", "1", "yes", "y", "on"}:
        return True
    if normalized in {"false", "0", "no", "n", "off"}:
        return False
    return default


def _allocate_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def _wait_for_ws(port: int, timeout_ms: int) -> str:
    deadline = time.time() + (timeout_ms / 1000.0)
    url = f"http://127.0.0.1:{port}/json/version"
    last_error: Optional[Exception] = None

    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=1) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                endpoint = (
                    data.get("webSocketDebuggerUrl")
                    or data.get("webSocketDebuggerUrl".lower())
                    or data.get("webSocketDebuggerUrl".upper())
                )
                if endpoint:
                    return endpoint
        except (urllib.error.URLError, json.JSONDecodeError, TimeoutError, ConnectionError) as exc:
            last_error = exc
            time.sleep(0.1)
        except Exception as exc:  # pragma: no cover - unexpected errors are propagated
            last_error = exc
            time.sleep(0.1)

    if last_error:
        raise RuntimeError(f"timed out waiting for wsEndpoint: {last_error}") from last_error
    raise RuntimeError("timed out waiting for wsEndpoint")


def _chromium_executable(headless: bool) -> str:
    with sync_playwright() as playwright:
        browser_type = playwright.chromium
        exe_path = browser_type.executable_path
        if not exe_path:
            raise RuntimeError("unable to determine Chromium executable path")
        return exe_path


def _launch_chromium(args: argparse.Namespace) -> Dict[str, Any]:
    port = _allocate_port()
    headless = _parse_bool(args.headless, True)
    timeout_ms = args.launch_timeout if args.launch_timeout > 0 else 30000

    user_data_dir = args.user_data_dir
    if not user_data_dir:
        user_data_dir = tempfile.mkdtemp(prefix="rocketship-playwright-")
    os.makedirs(user_data_dir, exist_ok=True)

    executable = _chromium_executable(headless)

    chrome_args = [
        executable,
        f"--remote-debugging-port={port}",
        f"--user-data-dir={user_data_dir}",
        "--no-first-run",
        "--no-default-browser-check",
        "--disable-background-networking",
        "--disable-component-update",
        "--disable-device-discovery-notifications",
        "--disable-domain-reliability",
        "--disable-features=Translate",
        "--disable-renderer-backgrounding",
        "--disable-sync",
        "--metrics-recording-only",
        "--enable-automation",
        "--password-store=basic",
        "about:blank",
    ]

    if headless:
        chrome_args.insert(1, "--headless=new")

    if args.launch_arg:
        chrome_args.extend(args.launch_arg)

    env = os.environ.copy()
    env["PLAYWRIGHT_BROWSERS_PATH"] = env.get("PLAYWRIGHT_BROWSERS_PATH", "0")

    process = subprocess.Popen(
        chrome_args,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env=env,
        start_new_session=True,
    )

    try:
        ws_endpoint = _wait_for_ws(port, timeout_ms)
    except Exception:
        process.terminate()
        process.wait(timeout=5)
        raise

    return {
        "ok": True,
        "wsEndpoint": ws_endpoint,
        "pid": process.pid,
        "userDataDir": user_data_dir,
        "port": port,
    }


def _run_start(args: argparse.Namespace) -> None:
    try:
        payload = _launch_chromium(args)
        _write(payload)
    except Exception as exc:
        _write({"ok": False, "error": str(exc)})
        sys.exit(1)


def _load_script(path: str) -> str:
    with open(path, "r", encoding="utf-8") as handle:
        return handle.read()


def _run_script(args: argparse.Namespace) -> None:
    env_vars: Dict[str, Any] = {}
    if args.env_json:
        try:
            env_vars = json.loads(args.env_json)
        except json.JSONDecodeError as exc:
            _write({"ok": False, "error": f"failed to decode env: {exc}"})
            sys.exit(1)

    playwright = sync_playwright().start()

    try:
        browser = playwright.chromium.connect_over_cdp(args.ws_endpoint)
        contexts = browser.contexts
        if contexts:
            context = contexts[0]
        else:
            context = browser.new_context()

        pages = context.pages
        if pages:
            page = pages[0]
        else:
            page = context.new_page()

        script_source = _load_script(args.script_file)

        globals_dict: Dict[str, Any] = {
            "__name__": "__main__",
            "playwright": playwright,
            "browser": browser,
            "context": context,
            "page": page,
            "env": env_vars,
            "result": None,
        }

        try:
            exec(script_source, globals_dict, globals_dict)  # noqa: S102 - intentional exec for user script
        except Exception:
            error_text = traceback.format_exc()
            _write({"ok": False, "error": error_text})
            sys.exit(1)

        _write({"ok": True, "result": globals_dict.get("result")})
    finally:
        try:
            playwright.stop()
        except Exception:
            pass


def main() -> None:
    parser = argparse.ArgumentParser("playwright_runner")
    subparsers = parser.add_subparsers(dest="command", required=True)

    start_parser = subparsers.add_parser("start")
    start_parser.add_argument("--headless", default="true")
    start_parser.add_argument("--slow-mo-ms", type=int, default=0)
    start_parser.add_argument("--launch-arg", action="append")
    start_parser.add_argument("--launch-timeout", type=int, default=30000)
    start_parser.add_argument("--user-data-dir")

    script_parser = subparsers.add_parser("script")
    script_parser.add_argument("--ws-endpoint", required=True)
    script_parser.add_argument("--script-file", required=True)
    script_parser.add_argument("--env-json", default="")

    args = parser.parse_args()

    if args.command == "start":
        _run_start(args)
    elif args.command == "script":
        _run_script(args)
    else:  # pragma: no cover - argparse prevents invalid commands
        parser.error(f"Unknown command: {args.command}")


if __name__ == "__main__":
    main()
