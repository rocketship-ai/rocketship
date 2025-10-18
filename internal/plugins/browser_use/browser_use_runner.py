import argparse
import asyncio
import inspect
import json
import logging
import sys
import traceback

# Configure logging BEFORE importing browser-use to ensure logs go to stderr
logging.basicConfig(
    stream=sys.stderr,
    level=logging.INFO,
    format='%(levelname)-8s [%(name)s] %(message)s',
    force=True
)

try:
    from browser_use import Agent, BrowserSession
    from pydantic import BaseModel, Field
    from typing import Literal
except ImportError as exc:
    sys.stdout.write(json.dumps({"ok": False, "error": f"browser-use not available: {exc}"}) + "\n")
    sys.exit(1)


# Structured output schema for QA testing (qa-use pattern)
class BrowserTestResult(BaseModel):
    """
    Structured output schema for browser-based QA testing.
    Forces the agent to return machine-readable pass/fail status.
    """
    status: Literal["pass", "fail"] = Field(
        description="Test status - 'pass' if all criteria met, 'fail' otherwise"
    )
    message: str = Field(
        description="Description of what was verified or what went wrong"
    )
    error: str | None = Field(
        default=None,
        description="Detailed error message if status is 'fail', otherwise null"
    )
    extracted_data: dict | None = Field(
        default=None,
        description="Any data extracted during test execution"
    )


# QA-focused system prompt for structured output
QA_TESTING_SYSTEM_PROMPT = """
You are a QA testing agent that validates web application behavior.

CRITICAL: You MUST return your response as a JSON object with this exact structure:
{
  "status": "pass" or "fail",
  "message": "description of what you verified",
  "error": "detailed error if failed, otherwise null",
  "extracted_data": "any data you extracted, otherwise null"
}

Set status to "pass" ONLY if you successfully completed the task and verified all requirements.
Set status to "fail" if you:
- Encountered errors or exceptions
- Could not find required elements
- Could not complete the task as specified
- Partially completed the task but not fully

Be specific and detailed in your message and error fields so users can understand what happened.
"""


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

    session = None
    try:
        session = BrowserSession(cdp_url=args.ws_endpoint)
        start_fn = getattr(session, "start", None)
        if callable(start_fn):
            maybe_coro = start_fn()
            if inspect.isawaitable(maybe_coro):
                await maybe_coro

        # CRITICAL FIX: Re-focus to the correct target after external CDP changes (Playwright handoff)
        # After start(), session.connect() picks the FIRST target from getTargets(), which may not be
        # the active tab left open by Playwright. We need to find and switch to the most recently active target.
        if hasattr(session, 'get_most_recently_opened_target_id') and hasattr(session, 'event_bus'):
            from browser_use.browser.events import SwitchTabEvent

            try:
                # DEBUG: Dump all available targets
                if hasattr(session, '_cdp_get_all_pages'):
                    all_targets = await session._cdp_get_all_pages(include_http=True, include_about=True, include_pages=True, include_iframes=False, include_workers=False)
                    sys.stderr.write(f"[HANDOFF-DEBUG] Found {len(all_targets)} page targets:\n")
                    for idx, target in enumerate(all_targets):
                        target_id = target.get('targetId', 'unknown')
                        url = target.get('url', 'unknown')
                        target_type = target.get('type', 'unknown')
                        sys.stderr.write(f"[HANDOFF-DEBUG]   [{idx}] ID={target_id[-12:]}, type={target_type}, url={url[:80]}\n")
                    sys.stderr.flush()

                # Get the most recently opened/active target (last in the list)
                active_target_id = await session.get_most_recently_opened_target_id()

                # Only switch if it's different from current focus
                current_target = session.agent_focus.target_id if session.agent_focus else None
                sys.stderr.write(f"[HANDOFF-DEBUG] Current focus: {current_target}\n")
                sys.stderr.write(f"[HANDOFF-DEBUG] Proposed active target: {active_target_id}\n")
                sys.stderr.flush()

                if active_target_id != current_target:
                    sys.stderr.write(f"[HANDOFF-DEBUG] Re-focusing from {current_target[-12:] if current_target else 'None'} to {active_target_id[-12:]}\n")
                    sys.stderr.flush()
                    logging.info(f"Re-focusing from target {current_target[-4:] if current_target else 'None'} to active target {active_target_id[-4:]}")
                    # Dispatch SwitchTabEvent to properly update agent_focus and all watchdogs
                    switch_event = session.event_bus.dispatch(SwitchTabEvent(target_id=active_target_id))
                    await switch_event
                    sys.stderr.write(f"[HANDOFF-DEBUG] Successfully switched to target {active_target_id[-12:]}\n")
                    sys.stderr.flush()
                    logging.info(f"Successfully switched to target {active_target_id[-4:]}")
                else:
                    sys.stderr.write("[HANDOFF-DEBUG] No switch needed - already on active target\n")
                    sys.stderr.flush()
            except Exception as refocus_err:
                sys.stderr.write(f"[HANDOFF-DEBUG] ERROR during refocus: {refocus_err}\n")
                sys.stderr.flush()
                logging.warning(f"Failed to re-focus to active target (continuing anyway): {refocus_err}")
                # Continue execution - the initial target might still work

    except Exception as exc:
        logging.warning("BrowserSession attach failed; falling back to Browser: %s", exc)
        # Fallback to Browser for older versions or attachment failures
        from browser_use import Browser

        browser = Browser(cdp_url=args.ws_endpoint, keep_alive=True)
        await browser.start()
        session = browser

        # Apply same refocus logic to Browser fallback path
        if hasattr(session, 'get_most_recently_opened_target_id') and hasattr(session, 'event_bus'):
            from browser_use.browser.events import SwitchTabEvent

            try:
                active_target_id = await session.get_most_recently_opened_target_id()
                current_target = session.agent_focus.target_id if session.agent_focus else None
                logging.debug(f"Fallback refocus check: current={current_target}, active={active_target_id}")

                if active_target_id != current_target:
                    logging.info(f"Fallback: Re-focusing from target {current_target[-4:] if current_target else 'None'} to active target {active_target_id[-4:]}")
                    switch_event = session.event_bus.dispatch(SwitchTabEvent(target_id=active_target_id))
                    await switch_event
                    logging.info(f"Fallback: Successfully switched to target {active_target_id[-4:]}")
            except Exception as refocus_err:
                logging.warning(f"Fallback: Failed to re-focus to active target (continuing anyway): {refocus_err}")

    agent_kwargs = {
        "task": args.task,
        "llm": llm,
        "browser_session": session,
        # Use structured output schema (qa-use pattern)
        "output_model_schema": BrowserTestResult,
        # QA-focused system prompt
        "extend_system_message": QA_TESTING_SYSTEM_PROMPT,
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

        # Parse structured output (qa-use pattern)
        from pydantic import ValidationError

        try:
            # Extract and validate structured output from agent
            test_result = BrowserTestResult.model_validate_json(result.final_result())

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

            # Build payload from structured output
            payload = {
                "ok": test_result.status == "pass",
                "result": _serialize(result),
                "finalUrl": final_url,
                "message": test_result.message,
            }

            # Add error details if test failed
            if test_result.status == "fail":
                payload["error"] = test_result.error or test_result.message

            # Add extracted data if present
            if test_result.extracted_data:
                payload["extracted_data"] = test_result.extracted_data

            _write(payload)

        except (ValidationError, json.JSONDecodeError) as e:
            # Graceful fallback if structured output parsing fails
            # This should rarely happen with native LLM enforcement

            # Extract final URL even in fallback case
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
                pass

            payload = {
                "ok": False,
                "error": f"Agent returned invalid response format: {str(e)[:200]}",
                "result": _serialize(result),
                "finalUrl": final_url,
            }
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
