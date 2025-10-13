import argparse
import json
import os
import signal
import sys
import time
import traceback

try:
    from playwright.sync_api import sync_playwright
except ImportError as exc:
    sys.stderr.write(f"playwright not available: {exc}\n")
    sys.stdout.write(json.dumps({"ok": False, "error": f"playwright not available: {exc}"}) + "\n")
    sys.exit(1)


def _write_response(payload: dict) -> None:
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


def _parse_bool(value: str, default: bool = True) -> bool:
    if value is None:
        return default
    value = value.strip().lower()
    if value in {"true", "1", "yes", "y", "on"}:
        return True
    if value in {"false", "0", "no", "n", "off"}:
        return False
    return default


def _run_start(args: argparse.Namespace) -> None:
    playwright = sync_playwright().start()

    launch_kwargs = {
        "headless": _parse_bool(args.headless, True),
        "timeout": args.launch_timeout,
    }

    if args.slow_mo_ms > 0:
        launch_kwargs["slow_mo"] = args.slow_mo_ms

    if args.launch_arg:
        launch_kwargs["args"] = args.launch_arg

    try:
        browser_server = playwright.chromium.launch_server(**launch_kwargs)
    except Exception:
        playwright.stop()
        raise

    payload = {
        "ok": True,
        "wsEndpoint": browser_server.ws_endpoint,
        "pid": os.getpid(),
        "browserPID": browser_server.process.pid,
    }
    _write_response(payload)

    def _shutdown_handler(_signum, _frame):
        try:
            browser_server.close()
        except Exception:
            pass
        try:
            playwright.stop()
        except Exception:
            pass
        sys.exit(0)

    signal.signal(signal.SIGTERM, _shutdown_handler)
    signal.signal(signal.SIGINT, _shutdown_handler)

    # Keep process alive so BrowserServer continues running
    while True:
        time.sleep(1)


def _load_script(path: str) -> str:
    with open(path, "r", encoding="utf-8") as script_file:
        return script_file.read()


def _run_script(args: argparse.Namespace) -> None:
    playwright = sync_playwright().start()
    try:
        browser = playwright.chromium.connect_over_cdp(args.ws_endpoint)
        if browser.contexts:
            context = browser.contexts[0]
        else:
            context = browser.new_context()

        if context.pages:
            page = context.pages[0]
        else:
            page = context.new_page()

        script_source = _load_script(args.script_file)

        env_vars = {}
        if args.env_json:
            env_vars = json.loads(args.env_json)

        exec_globals = {
            "__name__": "__main__",
            "browser": browser,
            "context": context,
            "page": page,
            "env": env_vars,
        }

        try:
            exec(script_source, exec_globals, exec_globals)
            result_payload = {"ok": True}
            if "result" in exec_globals:
                result_payload["result"] = exec_globals["result"]
            _write_response(result_payload)
        except Exception:
            error_text = traceback.format_exc()
            _write_response({"ok": False, "error": error_text})
            sys.exit(1)

    finally:
        try:
            playwright.stop()
        except Exception:
            pass


def main():
    parser = argparse.ArgumentParser("playwright_runner")
    subparsers = parser.add_subparsers(dest="command", required=True)

    start_parser = subparsers.add_parser("start")
    start_parser.add_argument("--headless", default="true")
    start_parser.add_argument("--slow-mo-ms", type=int, default=0)
    start_parser.add_argument("--launch-arg", action="append")
    start_parser.add_argument("--launch-timeout", type=int, default=0)

    script_parser = subparsers.add_parser("script")
    script_parser.add_argument("--ws-endpoint", required=True)
    script_parser.add_argument("--script-file", required=True)
    script_parser.add_argument("--env-json", default="")

    args = parser.parse_args()

    if args.command == "start":
        _run_start(args)
    elif args.command == "script":
        _run_script(args)
    else:
        parser.error(f"Unknown command: {args.command}")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        _write_response({"ok": False, "error": str(exc)})
        sys.exit(1)
