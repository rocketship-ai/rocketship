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

        # Debug: Print result info
        import sys
        print(f"DEBUG: task_completed={task_completed}", file=sys.stderr)
        print(f"DEBUG: result type={type(result)}", file=sys.stderr)
        if hasattr(result, 'final_result'):
            print(f"DEBUG: final_result={result.final_result()}", file=sys.stderr)
        if hasattr(result, 'errors'):
            print(f"DEBUG: errors={result.errors()}", file=sys.stderr)

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
            payload["error"] = "Task did not complete within max_steps limit"
        _write(payload)
    except Exception:
        error_text = traceback.format_exc()
        _write({"ok": False, "error": error_text})


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
