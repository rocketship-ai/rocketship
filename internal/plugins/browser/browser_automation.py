#!/usr/bin/env python3
"""
Browser automation script for Rocketship browser plugin.
This script uses browser-use to perform AI-driven web automation.
"""

import asyncio
import json
import sys
import os
import traceback
from datetime import datetime


async def main():
    """Main execution function for browser automation."""
    try:
        print("Starting browser automation...", file=sys.stderr)
        
        # Get configuration from environment variables
        task = os.environ.get('ROCKETSHIP_TASK', '')
        llm_provider = os.environ.get('ROCKETSHIP_LLM_PROVIDER', 'openai')
        llm_model = os.environ.get('ROCKETSHIP_LLM_MODEL', 'gpt-4')
        headless = os.environ.get('ROCKETSHIP_HEADLESS', 'true').lower() == 'true'
        browser_type = os.environ.get('ROCKETSHIP_BROWSER_TYPE', 'chromium')
        use_vision = os.environ.get('ROCKETSHIP_USE_VISION', 'true').lower() == 'true'
        max_steps = int(os.environ.get('ROCKETSHIP_MAX_STEPS', '50'))
        allowed_domains = os.environ.get('ROCKETSHIP_ALLOWED_DOMAINS', '').split(',') if os.environ.get('ROCKETSHIP_ALLOWED_DOMAINS') else []
        
        if not task:
            raise ValueError("Task is required but not provided")
        
        # Import browser-use
        try:
            from browser_use import Agent
            print("Successfully imported browser_use", file=sys.stderr)
        except ImportError as e:
            print(f"Failed to import browser_use: {e}", file=sys.stderr)
            raise
        
        # Initialize LLM based on provider
        llm = None
        if llm_provider == "openai":
            try:
                from langchain_openai import ChatOpenAI
                llm = ChatOpenAI(model=llm_model)
                print(f"OpenAI LLM initialized with model: {llm_model}", file=sys.stderr)
            except ImportError as e:
                print(f"Failed to import OpenAI: {e}", file=sys.stderr)
                print("Please install: pip install langchain-openai", file=sys.stderr)
                raise
        elif llm_provider == "anthropic":
            try:
                from langchain_anthropic import ChatAnthropic
                llm = ChatAnthropic(model=llm_model)
                print(f"Anthropic LLM initialized with model: {llm_model}", file=sys.stderr)
            except ImportError as e:
                print(f"Failed to import Anthropic: {e}", file=sys.stderr)
                print("Please install: pip install langchain-anthropic", file=sys.stderr)
                raise
        else:
            raise ValueError(f"Unsupported LLM provider: {llm_provider}")
        
        # Create browser agent
        print("Creating browser agent...", file=sys.stderr)
        
        # Create browser config if we have browser-specific settings
        browser_context = None
        try:
            from browser_use import BrowserConfig
            browser_context = BrowserConfig(
                headless=headless,
                browser_type=browser_type,
            )
            print(f"Browser config created: headless={headless}, type={browser_type}", file=sys.stderr)
        except (ImportError, TypeError) as e:
            print(f"BrowserConfig not available or incorrect parameters: {e}", file=sys.stderr)
        
        # Create agent with basic parameters (start simple and add features later)
        agent_kwargs = {
            'task': task,
            'llm': llm,
        }
        
        # Add browser context if available (try different parameter names)
        if browser_context:
            # Try browser_context first (as suggested by error message)
            agent_kwargs['browser_context'] = browser_context
            
        # Try to add optional parameters one by one
        try:
            agent = Agent(**agent_kwargs)
        except TypeError as e:
            print(f"Failed to create agent with full config, trying basic: {e}", file=sys.stderr)
            # Fall back to most basic configuration
            agent = Agent(task=task, llm=llm)
        print("Agent created successfully", file=sys.stderr)
        
        # Execute the task
        print("Starting task execution...", file=sys.stderr)
        print(f"Task: {task}", file=sys.stderr)
        result = await agent.run()
        print("Task execution completed", file=sys.stderr)
        
        # Build response
        response = {
            "success": True,
            "result": str(result) if result else "Task completed successfully",
            "session_id": "",
            "steps": [],
            "screenshots": [],
            "extracted_data": {}
        }
        
        # Try to extract any data that was found
        if hasattr(result, 'extracted_content') and result.extracted_content:
            response["extracted_data"] = result.extracted_content
            response["result"] = str(result.extracted_content)
            print(f"Extracted data: {result.extracted_content}", file=sys.stderr)
        
        # Try to get session information if available
        if hasattr(result, 'session_id'):
            response["session_id"] = result.session_id
        
        # Try to get step information if available
        if hasattr(result, 'steps'):
            response["steps"] = result.steps
        
        # Output the JSON response
        print(json.dumps(response))
        
    except Exception as e:
        print(f"Error during execution: {e}", file=sys.stderr)
        print(f"Traceback: {traceback.format_exc()}", file=sys.stderr)
        
        error_response = {
            "success": False,
            "error": str(e),
            "result": "",
            "session_id": "",
            "steps": [],
            "screenshots": [],
            "extracted_data": {}
        }
        print(json.dumps(error_response))
        sys.exit(1)


if __name__ == "__main__":
    print("Python script starting...", file=sys.stderr)
    asyncio.run(main())

