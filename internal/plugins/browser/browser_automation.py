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
        viewport_width = int(os.environ.get('ROCKETSHIP_VIEWPORT_WIDTH', '1920'))
        viewport_height = int(os.environ.get('ROCKETSHIP_VIEWPORT_HEIGHT', '1080'))
        
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
        
        # Create browser profile for headless mode
        browser_profile = None
        try:
            from browser_use import BrowserProfile
            from playwright._impl._api_structures import ViewportSize
            
            # Create profile with headless setting and browser channel
            profile_kwargs = {
                'headless': headless,
            }
            
            # Map browser_type to channel enum value
            if browser_type == 'chromium':
                from browser_use.browser.profile import BrowserChannel
                profile_kwargs['channel'] = BrowserChannel.CHROMIUM
            
            # Add viewport settings - try ViewportSize object which playwright expects
            profile_kwargs['viewport'] = ViewportSize(width=viewport_width, height=viewport_height)
            
            # For non-headless mode, also set window_size to control the browser window
            if not headless:
                profile_kwargs['window_size'] = ViewportSize(width=viewport_width, height=viewport_height)
            
            browser_profile = BrowserProfile(**profile_kwargs)
            print(f"Browser profile created: headless={headless}, type={browser_type}, viewport={viewport_width}x{viewport_height}", file=sys.stderr)
        except (ImportError, TypeError) as e:
            print(f"BrowserProfile not available or incorrect parameters: {e}", file=sys.stderr)
            # Try fallback with dict format
            try:
                profile_kwargs['viewport'] = {'width': viewport_width, 'height': viewport_height}
                browser_profile = BrowserProfile(**profile_kwargs)
                print(f"Browser profile created with dict viewport: headless={headless}, type={browser_type}, viewport={viewport_width}x{viewport_height}", file=sys.stderr)
            except Exception as e2:
                print(f"Failed with dict viewport too: {e2}", file=sys.stderr)
        
        # Create agent with browser profile
        agent_kwargs = {
            'task': task,
            'llm': llm,
            'use_vision': use_vision,
            'max_actions_per_step': max_steps,  # This maps to max_actions_per_step in Agent
        }
        
        # Add browser profile if available
        if browser_profile:
            agent_kwargs['browser_profile'] = browser_profile
            
        # Add allowed domains if specified (as a message context hint)
        if allowed_domains and allowed_domains[0]:  # Check if not empty
            domains_str = ', '.join(allowed_domains)
            agent_kwargs['message_context'] = f"Please only interact with the following domains: {domains_str}"
            print(f"Restricted to domains: {domains_str}", file=sys.stderr)
            
        # Create agent
        print("Creating agent with configuration...", file=sys.stderr)
        agent = Agent(**agent_kwargs)
        
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

