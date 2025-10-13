import argparse
import asyncio
import json
import sys
import traceback

try:
    from browser_use import Agent, BrowserSession
except ImportError as exc:
    sys.stdout.write(json.dumps({"ok": False, "error": f"browser-use not available: {exc}"}) + "\n")
    sys.exit(1)


def _write(payload: dict) -> None:
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


def _serialize(value):
    if value is None:
        return None
    if hasattr(value, "model_dump"):
        try:
            return value.model_dump()
        except Exception:
            pass
    if isinstance(value, dict):
        return value
    if isinstance(value, (list, tuple)):
        return [_serialize(item) for item in value]
    if hasattr(value, "__dict__"):
        try:
            return {k: _serialize(v) for k, v in value.__dict__.items() if not k.startswith("_")}
        except Exception:
            pass
    return str(value)


async def _run(args: argparse.Namespace) -> None:
    try:
        session = BrowserSession(cdp_url=args.ws_endpoint)
    except Exception:
        # Fallback to Browser for older versions
        from browser_use import Browser

        browser = Browser(cdp_url=args.ws_endpoint, keep_alive=True)
        await browser.start()
        session = browser

    agent_kwargs = {
        "task": args.task,
        "browser_session": session,
    }

    if args.allowed_domain:
        agent_kwargs["allowed_domains"] = args.allowed_domain

    if args.use_vision:
        agent_kwargs["use_vision"] = True

    if args.max_steps > 0:
        agent_kwargs["max_steps"] = args.max_steps

    if args.temperature is not None:
        agent_kwargs["temperature"] = args.temperature

    agent = Agent(**agent_kwargs)

    try:
        result = await agent.run(max_steps=args.max_steps if args.max_steps > 0 else None)
        payload = {
            "ok": True,
            "result": _serialize(result),
        }
        _write(payload)
    except Exception:
        error_text = traceback.format_exc()
        _write({"ok": False, "error": error_text})


def main() -> None:
    parser = argparse.ArgumentParser("browser_use_runner")
    parser.add_argument("--ws-endpoint", required=True)
    parser.add_argument("--task", required=True)
    parser.add_argument("--allowed-domain", action="append")
    parser.add_argument("--max-steps", type=int, default=0)
    parser.add_argument("--use-vision", action="store_true")
    parser.add_argument("--temperature", type=float)

    args = parser.parse_args()

    asyncio.run(_run(args))


if __name__ == "__main__":
    main()
