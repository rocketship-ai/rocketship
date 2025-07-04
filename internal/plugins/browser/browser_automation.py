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
import signal

# Configure logging to go to stderr so it doesn't interfere with JSON output on stdout
logging.basicConfig(
    level=logging.INFO,
    format='%(levelname)s     [%(name)s] %(message)s',
    stream=sys.stderr
)


# Global variable to track browser instance for cleanup
browser_instance = None
agent_instance = None

def signal_handler(signum, frame):
    """Handle termination signals to cleanup browser."""
    print(f"[DEBUG] Python signal handler received signal {signum} ({signal.Signals(signum).name})", file=sys.stderr)
    print(f"[DEBUG] Browser instance exists: {browser_instance is not None}", file=sys.stderr)
    print(f"[DEBUG] Agent instance exists: {agent_instance is not None}", file=sys.stderr)
    
    # Try to cleanup browser instance gracefully
    if browser_instance:
        try:
            print("[DEBUG] Attempting to cleanup browser using event loop", file=sys.stderr)
            # Use asyncio to run cleanup in the current event loop
            loop = asyncio.get_running_loop()
            loop.create_task(cleanup_browser())
            print("[DEBUG] Cleanup task created", file=sys.stderr)
        except RuntimeError as e:
            print(f"[DEBUG] No event loop running, cannot cleanup gracefully: {e}", file=sys.stderr)
            # If no event loop is running, just exit
            pass
    else:
        print("[DEBUG] No browser instance to cleanup", file=sys.stderr)
    
    print("[DEBUG] Exiting Python process with os._exit(1)", file=sys.stderr)
    # Force exit without waiting for cleanup
    os._exit(1)

async def cleanup_browser():
    """Cleanup browser instance."""
    global browser_instance, agent_instance
    
    print(f"[DEBUG] cleanup_browser() called - browser_instance: {browser_instance is not None}, agent_instance: {agent_instance is not None}", file=sys.stderr)
    
    # Try to cleanup browser through agent first
    if agent_instance:
        try:
            print("[DEBUG] Attempting to cleanup browser through agent...", file=sys.stderr)
            if hasattr(agent_instance, 'browser') and agent_instance.browser:
                print(f"[DEBUG] Found browser in agent: {type(agent_instance.browser)}", file=sys.stderr)
                await agent_instance.browser.close()
                print("[DEBUG] Browser closed through agent successfully.", file=sys.stderr)
            elif hasattr(agent_instance, '_browser') and agent_instance._browser:
                print(f"[DEBUG] Found _browser in agent: {type(agent_instance._browser)}", file=sys.stderr)
                await agent_instance._browser.close()
                print("[DEBUG] Browser closed through agent._browser successfully.", file=sys.stderr)
            else:
                print("[DEBUG] No browser attribute found in agent", file=sys.stderr)
        except Exception as e:
            print(f"[DEBUG] Error closing browser through agent: {e}", file=sys.stderr)
    
    # Fallback to global browser_instance
    if browser_instance:
        try:
            print(f"[DEBUG] Closing global browser instance: {type(browser_instance)}", file=sys.stderr)
            await browser_instance.close()
            print("[DEBUG] Global browser instance closed successfully.", file=sys.stderr)
        except Exception as e:
            print(f"[DEBUG] Error closing global browser: {e}", file=sys.stderr)
    else:
        print("[DEBUG] No global browser instance to close", file=sys.stderr)

async def main():
    """Main execution function for browser automation."""
    global browser_instance, agent_instance
    
    # Register signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    print(f"[DEBUG] Python script starting, PID: {os.getpid()}", file=sys.stderr)
    print("[DEBUG] Registered signal handlers for SIGINT and SIGTERM", file=sys.stderr)
    
    try:
        print("[DEBUG] Starting browser automation...", file=sys.stderr)
        
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
        }
        
        # Add browser profile if available
        if browser_profile:
            agent_kwargs['browser_profile'] = browser_profile
            
        # Add allowed domains if specified (as a message context hint)
        if allowed_domains and allowed_domains[0]:  # Check if not empty
            domains_str = ', '.join(allowed_domains)
            agent_kwargs['message_context'] = f"Please only interact with the following domains: {domains_str}"
            print(f"Restricted to domains: {domains_str}", file=sys.stderr)
        
        # Enhance task to ensure structured output
        enhanced_task = f"{task}\n\nIMPORTANT: Your response must be a valid JSON object with at least a 'success' field (boolean) indicating whether the task was completed successfully."
        agent_kwargs['task'] = enhanced_task
            
        # Create agent
        print("Creating agent with configuration...", file=sys.stderr)
        print(f"[DEBUG] Agent configuration: {agent_kwargs}", file=sys.stderr)
        print(f"[DEBUG] Browser profile: {browser_profile}", file=sys.stderr)
        print("[DEBUG] About to create Agent instance - this will prepare browser setup", file=sys.stderr)
        
        agent = Agent(**agent_kwargs)
        agent_instance = agent  # Store for cleanup
        
        print("[DEBUG] Agent instance created successfully", file=sys.stderr)
        
        # Execute the task with max_steps
        print("Starting task execution...", file=sys.stderr)
        print(f"Task: {task}", file=sys.stderr)
        print(f"Max steps: {max_steps}", file=sys.stderr)
        
        # Add debug logging before running the agent
        print("[DEBUG] About to start agent.run() - browser will be created internally", file=sys.stderr)
        
        result = await agent.run(max_steps=max_steps)
        
        # Add debug logging after agent run completes
        print("[DEBUG] Agent.run() completed - browser should now be active", file=sys.stderr)
        
        # Try to access browser instance from the agent for debugging
        try:
            if hasattr(agent, 'browser') and agent.browser:
                browser_instance = agent.browser
                print(f"[DEBUG] Browser instance accessed from agent: {type(browser_instance)}", file=sys.stderr)
                print("[DEBUG] Browser instance created successfully", file=sys.stderr)
            elif hasattr(agent, '_browser') and agent._browser:
                browser_instance = agent._browser
                print(f"[DEBUG] Browser instance accessed from agent._browser: {type(browser_instance)}", file=sys.stderr)
                print("[DEBUG] Browser instance created successfully", file=sys.stderr)
            else:
                print("[DEBUG] Could not access browser instance from agent - may be managed internally", file=sys.stderr)
                # Check for other possible browser attributes
                browser_attrs = [attr for attr in dir(agent) if 'browser' in attr.lower()]
                if browser_attrs:
                    print(f"[DEBUG] Available browser-related attributes on agent: {browser_attrs}", file=sys.stderr)
        except Exception as e:
            print(f"[DEBUG] Error accessing browser instance: {e}", file=sys.stderr)
            
        print("Task execution completed", file=sys.stderr)
        
        # Build response - check if the result indicates success
        success = True
        result_str = str(result) if result else "Task completed successfully"
        
        # Check if the result is a dict/object with a success field
        if hasattr(result, '__dict__') and hasattr(result, 'success'):
            success = bool(result.success)
            print(f"Result has success attribute: {success}", file=sys.stderr)
        elif isinstance(result, dict) and 'success' in result:
            success = bool(result['success'])
            print(f"Result dict has success key: {success}", file=sys.stderr)
        # If result is a string containing "success: false" or similar
        elif isinstance(result, str):
            result_lower = result.lower()
            if "success: false" in result_lower or "success\":false" in result_lower:
                success = False
                print(f"Result string indicates failure: {result}", file=sys.stderr)
            elif "failed" in result_lower or "error" in result_lower or "could not" in result_lower:
                success = False
                print(f"Result string contains failure indicators: {result}", file=sys.stderr)
        
        response = {
            "success": success,
            "result": result_str,
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
                    # If extracted content has success field, use it
                    if isinstance(extracted_content, dict) and 'success' in extracted_content:
                        response["success"] = bool(extracted_content['success'])
                        response["result"] = extracted_content
                    print(f"Extracted data: {extracted_content}", file=sys.stderr)
            except Exception as e:
                print(f"Error accessing extracted_content: {e}", file=sys.stderr)
        
        # If result is already a dict with structured data, use it as extracted_data
        if isinstance(result, dict):
            response["extracted_data"] = result
            if 'success' in result:
                response["success"] = bool(result['success'])
        
        # Try to get session information if available
        if hasattr(result, 'session_id'):
            response["session_id"] = result.session_id
        
        # Try to get step information if available
        if hasattr(result, 'steps'):
            response["steps"] = result.steps
        
        # Output the JSON response to stdout (separate from stderr debug logs)
        print(json.dumps(response), flush=True)
        
        # Final cleanup and debug logging
        print("[DEBUG] About to cleanup browser before exit", file=sys.stderr)
        await cleanup_browser()
        print("[DEBUG] Browser cleanup completed", file=sys.stderr)
        
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

