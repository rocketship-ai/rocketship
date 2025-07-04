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
    
    # Try to cleanup browser instance gracefully
    if browser_instance:
        try:
            # Use asyncio to run cleanup in the current event loop
            loop = asyncio.get_running_loop()
            loop.create_task(cleanup_browser())
        except RuntimeError:
            # If no event loop is running, just exit
            pass
    
    # Force exit without waiting for cleanup
    os._exit(1)

async def cleanup_browser():
    """Cleanup browser instance."""
    global browser_instance, agent_instance
    
    # Try to cleanup browser through agent first
    if agent_instance:
        try:
            if hasattr(agent_instance, 'browser') and agent_instance.browser:
                await agent_instance.browser.close()
            elif hasattr(agent_instance, '_browser') and agent_instance._browser:
                await agent_instance._browser.close()
        except Exception:
            pass
    
    # Fallback to global browser_instance
    if browser_instance:
        try:
            await browser_instance.close()
        except Exception:
            pass

async def main():
    """Main execution function for browser automation."""
    global browser_instance, agent_instance
    
    # Register signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    try:
        
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
        
        agent = Agent(**agent_kwargs)
        agent_instance = agent  # Store for cleanup
        
        # Execute the task with max_steps
        print("Starting task execution...", file=sys.stderr)
        print(f"Task: {task}", file=sys.stderr)
        print(f"Max steps: {max_steps}", file=sys.stderr)
        
        result = await agent.run(max_steps=max_steps)
        
        # Try to access browser instance from the agent for cleanup
        try:
            if hasattr(agent, 'browser') and agent.browser:
                browser_instance = agent.browser
            elif hasattr(agent, '_browser') and agent._browser:
                browser_instance = agent._browser
        except Exception:
            pass
            
        print("Task execution completed", file=sys.stderr)
        
        # Build response - check if the result indicates success
        success = True
        result_str = str(result) if result else "Task completed successfully"
        
        # Check if the result is a dict/object with a success field
        if hasattr(result, '__dict__') and hasattr(result, 'success'):
            success = bool(result.success)
        elif isinstance(result, dict) and 'success' in result:
            success = bool(result['success'])
        # If result is a string containing "success: false" or similar
        elif isinstance(result, str):
            result_lower = result.lower()
            if "success: false" in result_lower or "success\":false" in result_lower:
                success = False
            elif "failed" in result_lower or "error" in result_lower or "could not" in result_lower:
                success = False
        
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
        
        # Final cleanup
        await cleanup_browser()
        
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
    asyncio.run(main())

