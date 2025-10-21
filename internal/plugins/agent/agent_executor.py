#!/usr/bin/env python3
"""
Claude Agent SDK executor for Rocketship.
Supports MCP servers, sessions, and structured output.
"""
import argparse
import asyncio
import json
import logging
import os
import sys
import traceback
from typing import Any, Dict, Optional

# Configure logging to stderr BEFORE imports
logging.basicConfig(
    stream=sys.stderr,
    level=logging.INFO,
    format='%(levelname)-8s [%(name)s] %(message)s',
    force=True
)

logger = logging.getLogger(__name__)

try:
    from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions, AssistantMessage, TextBlock
except ImportError as exc:
    sys.stdout.write(json.dumps({"ok": False, "error": f"claude-agent-sdk not available: {exc}. Install with: pip install claude-agent-sdk"}) + "\n")
    sys.exit(1)


def _write(payload: dict) -> None:
    """Write JSON to stdout and flush"""
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


def _check_versions() -> Optional[dict]:
    """Check required package versions. Returns error dict if versions insufficient, None if OK."""
    from importlib.metadata import version, PackageNotFoundError

    required = {
        "claude-agent-sdk": "0.1.0",  # Minimum version
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
                "error": f"{package} not installed (version {min_version}+ required). Install with: pip install claude-agent-sdk"
            }
        except (ValueError, AttributeError):
            # Can't parse version, allow it through
            pass

    return None


def _extract_text_content(message: Any) -> str:
    """Extract text content from a Claude message"""
    if isinstance(message, AssistantMessage):
        text_parts = []
        for block in message.content:
            if isinstance(block, TextBlock):
                text_parts.append(block.text)
        return "\n".join(text_parts)
    return ""  # Skip system messages and metadata


def _parse_agent_result(response_text: str) -> dict:
    """
    Parse agent response to check if it contains test assertion results.
    If the agent returns JSON with {"ok": false}, we respect that to fail the test.
    Otherwise, treat the response as successful text generation.
    """
    import re

    # Try to extract JSON from markdown code blocks first
    # Pattern: ```json\n{...}\n``` or just {...}
    json_block_match = re.search(r'```(?:json)?\s*\n?(\{[\s\S]*?\})\s*\n?```', response_text)
    if json_block_match:
        json_text = json_block_match.group(1)
    else:
        # Look for raw JSON object in the text
        json_match = re.search(r'\{[\s\S]*\}', response_text)
        if json_match:
            json_text = json_match.group(0)
        else:
            json_text = response_text

    try:
        # Try to parse as JSON
        parsed = json.loads(json_text.strip())
        if isinstance(parsed, dict):
            # If agent returned structured result with ok/success field, respect it
            if "ok" in parsed or "success" in parsed:
                result = {
                    "ok": parsed.get("ok", parsed.get("success", True)),
                    "result": parsed.get("result", parsed.get("message", response_text)),
                    "error": parsed.get("error", "")
                }
                return result
    except (json.JSONDecodeError, ValueError):
        # Not JSON, treat as plain text (successful generation)
        pass

    # Default: treat response as successful text result
    return {
        "ok": True,
        "result": response_text,
        "error": ""
    }


async def _execute_agent(config: Dict[str, Any]) -> None:
    """Execute Claude agent with the given configuration"""
    logger.info(f"Executing agent with mode: {config.get('mode', 'single')}")

    # Parse MCP servers configuration
    mcp_servers = {}
    for name, server_config in config.get("mcp_servers", {}).items():
        server_type = server_config.get("type", "stdio")

        if server_type == "stdio":
            mcp_servers[name] = {
                "type": "stdio",
                "command": server_config["command"],
                "args": server_config.get("args", []),
                "env": server_config.get("env", {})
            }
            logger.info(f"Configured stdio MCP server: {name} -> {server_config['command']}")

        elif server_type == "sse":
            mcp_servers[name] = {
                "type": "sse",
                "url": server_config["url"],
                "headers": server_config.get("headers", {})
            }
            logger.info(f"Configured SSE MCP server: {name} -> {server_config['url']}")

        else:
            _write({"ok": False, "error": f"Unsupported MCP server type: {server_type}"})
            return

    # Parse tool permissions
    allowed_tools = config.get("allowed_tools", [])
    if allowed_tools == ["*"] or allowed_tools == "*":
        # Wildcard - allow all tools (don't specify allowed_tools to SDK)
        allowed_tools = None
        logger.info("Tool permissions: wildcard (all tools allowed)")
    elif allowed_tools:
        logger.info(f"Tool permissions: {len(allowed_tools)} specific tools allowed")
    else:
        # Empty list means no tools allowed (SDK default)
        logger.info("Tool permissions: no tools allowed (default)")

    # Build ClaudeAgentOptions
    options_kwargs = {
        "mcp_servers": mcp_servers if mcp_servers else None,
    }

    if allowed_tools is not None:
        options_kwargs["allowed_tools"] = allowed_tools

    # Always use bypassPermissions for QA testing - the agent should never ask for user permission
    # or modify files. It's job is to execute test tasks using MCP tools and return pass/fail results.
    options_kwargs["permission_mode"] = "bypassPermissions"

    if config.get("system_prompt"):
        options_kwargs["system_prompt"] = config["system_prompt"]

    if config.get("cwd"):
        options_kwargs["cwd"] = config["cwd"]

    # Create options
    options = ClaudeAgentOptions(**options_kwargs) if options_kwargs else None

    # Execute based on mode
    mode = config.get("mode", "single")
    prompt = config["prompt"]

    try:
        if mode == "single":
            # Single execution mode - use query() function for simplicity
            from claude_agent_sdk import query

            logger.info("Starting single execution")
            response_texts = []

            async for message in query(prompt=prompt, options=options):
                text = _extract_text_content(message)
                if text:
                    response_texts.append(text)
                    logger.debug(f"Received message: {text[:100]}...")

            final_response = "\n".join(response_texts)

            # Try to parse agent response as JSON to check for test assertions
            # If agent returns {"ok": false, ...}, respect that to fail the test
            agent_result = _parse_agent_result(final_response)

            _write({
                "ok": agent_result.get("ok", True),
                "result": agent_result.get("result", final_response),
                "error": agent_result.get("error", ""),
                "mode": "single"
            })

        elif mode in ["continue", "resume"]:
            # Session-based execution using ClaudeSDKClient
            session_id = config.get("session_id")

            if mode == "resume" and not session_id:
                _write({"ok": False, "error": "session_id is required for resume mode"})
                return

            logger.info(f"Starting {mode} execution" + (f" with session {session_id}" if session_id else ""))

            async with ClaudeSDKClient(options=options) as client:
                # For resume mode, set the session ID
                if mode == "resume" and session_id:
                    client.session_id = session_id

                # Send the query
                await client.query(prompt)

                # Receive and collect responses
                response_texts = []
                async for message in client.receive_response():
                    text = _extract_text_content(message)
                    if text:
                        response_texts.append(text)
                        logger.debug(f"Received message: {text[:100]}...")

                final_response = "\n".join(response_texts)

                # Try to parse agent response as JSON to check for test assertions
                agent_result = _parse_agent_result(final_response)

                _write({
                    "ok": agent_result.get("ok", True),
                    "result": agent_result.get("result", final_response),
                    "error": agent_result.get("error", ""),
                    "session_id": client.session_id,
                    "mode": mode
                })

        else:
            _write({"ok": False, "error": f"Unsupported mode: {mode}"})

    except Exception as exc:
        logger.error(f"Agent execution failed: {exc}")
        tb = traceback.format_exc()
        logger.error(f"Traceback:\n{tb}")
        _write({
            "ok": False,
            "error": str(exc),
            "traceback": tb
        })


async def main():
    parser = argparse.ArgumentParser(description="Claude Agent SDK executor")
    parser.add_argument("--config-json", required=True, help="JSON configuration for agent execution")
    args = parser.parse_args()

    # Check versions first
    version_error = _check_versions()
    if version_error:
        _write(version_error)
        return

    # Check for ANTHROPIC_API_KEY
    if not os.environ.get("ANTHROPIC_API_KEY"):
        _write({"ok": False, "error": "ANTHROPIC_API_KEY environment variable is required"})
        return

    # Parse configuration
    try:
        config = json.loads(args.config_json)
    except json.JSONDecodeError as exc:
        _write({"ok": False, "error": f"Invalid JSON configuration: {exc}"})
        return

    # Validate required fields
    if "prompt" not in config:
        _write({"ok": False, "error": "prompt is required in configuration"})
        return

    # Execute agent
    await _execute_agent(config)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Agent execution interrupted")
        sys.exit(130)
    except Exception as exc:
        logger.error(f"Fatal error: {exc}")
        _write({"ok": False, "error": f"Fatal error: {exc}"})
        sys.exit(1)
