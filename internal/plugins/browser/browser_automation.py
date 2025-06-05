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
import logging
from datetime import datetime

# Configure logging to go to stderr so it doesn't interfere with JSON output on stdout
logging.basicConfig(
    level=logging.INFO,
    format='%(levelname)s     [%(name)s] %(message)s',
    stream=sys.stderr
)


async def main():
    """Main execution function for browser automation."""
    try:
        print("Starting browser automation...", file=sys.stderr)
        
        # Get configuration from environment variables
        task = os.environ.get('ROCKETSHIP_TASK', '')
        llm_provider = os.environ.get('ROCKETSHIP_LLM_PROVIDER', 'openai')
        llm_model = os.environ.get('ROCKETSHIP_LLM_MODEL', 'gpt-4o')
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
                
                # Check if API key is available
                api_key = os.environ.get('OPENAI_API_KEY')
                if not api_key:
                    raise ValueError("OPENAI_API_KEY environment variable is required")
                
                print(f"Found OpenAI API key: {api_key[:20]}...", file=sys.stderr)
                
                # Initialize with explicit API key and better model name
                llm = ChatOpenAI(
                    model=llm_model,
                    openai_api_key=api_key,
                    timeout=30,
                    max_retries=2
                )
                print(f"OpenAI LLM initialized with model: {llm_model}", file=sys.stderr)
                
                # Test the connection
                print("Testing LLM connection...", file=sys.stderr)
                test_response = llm.invoke("Hello")
                print(f"LLM connection test successful: {test_response.content[:50]}...", file=sys.stderr)
                
            except ImportError as e:
                print(f"Failed to import OpenAI: {e}", file=sys.stderr)
                print("Please install: pip install langchain-openai", file=sys.stderr)
                raise
            except Exception as e:
                print(f"Failed to initialize or test OpenAI LLM: {e}", file=sys.stderr)
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
            
        # Try to create agent with fallback strategies
        agent = None
        for attempt in range(3):
            try:
                if attempt == 0:
                    # Try with browser context
                    print("Attempting to create agent with browser context...", file=sys.stderr)
                    agent = Agent(**agent_kwargs)
                elif attempt == 1:
                    # Try without browser context
                    print("Retrying without browser context...", file=sys.stderr)
                    simple_kwargs = {'task': task, 'llm': llm}
                    agent = Agent(**simple_kwargs)
                else:
                    # Last resort - most basic
                    print("Last resort: most basic agent configuration...", file=sys.stderr)
                    agent = Agent(task=task, llm=llm)
                
                if agent:
                    print(f"Agent created successfully on attempt {attempt + 1}", file=sys.stderr)
                    break
                    
            except Exception as e:
                print(f"Attempt {attempt + 1} failed: {e}", file=sys.stderr)
                if attempt == 2:  # Last attempt
                    raise e
                continue
        
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
        if hasattr(result, 'extracted_content'):
            try:
                # Handle if extracted_content is a method or property
                extracted_content = result.extracted_content() if callable(result.extracted_content) else result.extracted_content
                if extracted_content:
                    response["extracted_data"] = extracted_content
                    response["result"] = str(extracted_content)
                    print(f"Extracted data: {extracted_content}", file=sys.stderr)
            except Exception as e:
                print(f"Error accessing extracted_content: {e}", file=sys.stderr)
        
        # Try to get session information if available
        if hasattr(result, 'session_id'):
            response["session_id"] = result.session_id
        
        # Try to get step information if available
        if hasattr(result, 'steps'):
            response["steps"] = result.steps
        
        # Output the JSON response to stdout (separate from stderr debug logs)
        print(json.dumps(response), flush=True)
        
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

