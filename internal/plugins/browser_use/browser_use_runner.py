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


def _check_versions() -> dict | None:
    """Check required package versions. Returns error dict if versions insufficient, None if OK."""
    from importlib.metadata import version, PackageNotFoundError

    required = {
        "browser-use": "0.8.0",
        "playwright": "1.40.0",
    }

    for package, min_version in required.items():
        try:
            installed = version(package)
            # Simple version comparison (works for semver)
            installed_parts = [int(x) for x in installed.split(".")[:3]]
            required_parts = [int(x) for x in min_version.split(".")[:3]]

            if installed_parts < required_parts:
                return {
                    "ok": False,
                    "error": f"{package} version {min_version}+ required, found {installed}"
                }
        except PackageNotFoundError:
            return {
                "ok": False,
                "error": f"{package} not installed (version {min_version}+ required)"
            }
        except (ValueError, AttributeError):
            # Can't parse version, allow it through
            pass

    return None


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
    # Check versions first
    version_error = _check_versions()
    if version_error:
        _write(version_error)
        return

    # Initialize LLM using browser-use's Chat classes (imported from browser_use directly)
    llm = None
    import os

    if args.llm_provider == "openai":
        from browser_use import ChatOpenAI

        api_key = os.environ.get("OPENAI_API_KEY")
        if not api_key:
            _write({"ok": False, "error": "OPENAI_API_KEY environment variable required"})
            return

        llm = ChatOpenAI(
            model=args.llm_model or "gpt-4o",
            api_key=api_key,
            timeout=30,
            max_retries=2
        )
    elif args.llm_provider == "anthropic":
        from browser_use import ChatAnthropic

        api_key = os.environ.get("ANTHROPIC_API_KEY")
        if not api_key:
            _write({"ok": False, "error": "ANTHROPIC_API_KEY environment variable required"})
            return

        llm = ChatAnthropic(
            model=args.llm_model or "claude-3-5-sonnet-20241022",
            api_key=api_key
        )
    else:
        _write({"ok": False, "error": f"Unsupported LLM provider: {args.llm_provider}"})
        return

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
        "llm": llm,
        "browser_session": session,
        # Add system message to ensure agent treats incomplete tasks as failures
        "extend_system_message": (
            "\n\nIMPORTANT: You MUST complete the full task as specified. "
            "If you cannot complete ANY part of the task due to missing elements, "
            "errors, or inability to find required content, you should report this as a failure. "
            "Do NOT report success if you only partially completed the task or could not find required elements."
        ),
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

        # Check if task actually completed
        task_completed = result.is_done() if hasattr(result, 'is_done') else False

        # Additionally check if the final result indicates failure
        error_message = None
        if hasattr(result, 'final_result'):
            final_text = str(result.final_result()).lower()
            # Check for common failure indicators in the agent's response
            failure_indicators = [
                'unable to',
                'could not',
                'couldn\'t',
                'cannot',
                'can\'t',
                'did not find',
                'didn\'t find',
                'not found',
                'failed to',
                'failure',
            ]
            if any(indicator in final_text for indicator in failure_indicators):
                task_completed = False
                error_message = f"Agent reported task failure: {result.final_result()}"

        # Check for errors in the result
        if hasattr(result, 'errors'):
            errors = result.errors()
            if errors and any(e is not None for e in errors):
                task_completed = False
                if not error_message:
                    error_message = f"Agent encountered errors: {errors}"

        # Extract final URL from browser context
        final_url = ""
        try:
            if hasattr(session, "context") and session.context:
                pages = session.context.pages
                if pages and len(pages) > 0:
                    final_url = pages[0].url
            elif hasattr(session, "browser") and hasattr(session.browser, "contexts"):
                contexts = session.browser.contexts
                if contexts and len(contexts) > 0:
                    pages = contexts[0].pages
                    if pages and len(pages) > 0:
                        final_url = pages[0].url
        except Exception:
            # If we can't get URL, just leave it empty
            pass

        payload = {
            "ok": task_completed,
            "result": _serialize(result),
            "finalUrl": final_url,
        }
        if not task_completed:
            payload["error"] = error_message or "Task did not complete within max_steps limit"
        _write(payload)
    except Exception:
        exc_type, exc_value, exc_tb = sys.exc_info()
        short_error = "".join(traceback.format_exception_only(exc_type, exc_value)).strip()
        full_trace = "".join(traceback.format_exception(exc_type, exc_value, exc_tb))
        _write({"ok": False, "error": short_error, "traceback": full_trace})
        raise SystemExit(1)


def main() -> None:
    parser = argparse.ArgumentParser("browser_use_runner")
    parser.add_argument("--ws-endpoint", required=True)
    parser.add_argument("--task", required=True)
    parser.add_argument("--llm-provider", required=True)
    parser.add_argument("--llm-model")
    parser.add_argument("--allowed-domain", action="append")
    parser.add_argument("--max-steps", type=int, default=0)
    parser.add_argument("--use-vision", action="store_true")
    parser.add_argument("--temperature", type=float)

    args = parser.parse_args()

    asyncio.run(_run(args))


if __name__ == "__main__":
    main()
