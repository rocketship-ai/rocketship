"""Rocketship MCP Server implementation."""

import os
import asyncio
from pathlib import Path

from mcp.server import Server
from mcp.server.models import InitializationOptions
from mcp.server.stdio import stdio_server
from mcp.types import (
    TextContent,
    Tool,
)

from .generators import RocketshipTestGenerator
from .utils import RocketshipCLIWrapper, GitAnalyzer, ensure_rocketship_directory, save_yaml_file
from .types import CodebaseAnalysis


class RocketshipMCPServer:
    """MCP server for Rocketship test generation and management."""
    
    def __init__(self):
        self.server = Server("rocketship")
        self.generator = RocketshipTestGenerator()
        self.cli_wrapper = RocketshipCLIWrapper()
        self.git_analyzer = GitAnalyzer()
        
        # Register tool handlers
        self._register_tools()
    
    def _register_tools(self):
        """Register all MCP tools."""
        
        @self.server.list_tools()
        async def handle_list_tools() -> list[Tool]:
            """List available tools."""
            return [
                Tool(
                    name="scan_and_generate_test_suite",
                    description="Analyze codebase context and generate comprehensive test suite structure",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "project_root": {
                                "type": "string",
                                "description": "Root directory of the project (default: current directory)",
                                "default": "."
                            },
                            "environments": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "Target environments for test configuration",
                                "default": ["staging", "prod"]
                            },
                            "codebase_analysis": {
                                "type": "object",
                                "description": "Analysis of the codebase from agent context",
                                "properties": {
                                    "api_endpoints": {
                                        "type": "array",
                                        "items": {"type": "object"},
                                        "description": "Detected API endpoints with method and path"
                                    },
                                    "database_schemas": {
                                        "type": "array", 
                                        "items": {"type": "object"},
                                        "description": "Database schema information"
                                    },
                                    "service_configs": {
                                        "type": "array",
                                        "items": {"type": "object"},
                                        "description": "Service configuration details"
                                    },
                                    "environment_files": {
                                        "type": "array",
                                        "items": {"type": "string"},
                                        "description": "Environment configuration files found"
                                    }
                                },
                                "required": []
                            }
                        },
                        "required": ["codebase_analysis"]
                    }
                ),
                Tool(
                    name="generate_test_from_prompt",
                    description="Generate a Rocketship test file from natural language prompt",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "prompt": {
                                "type": "string",
                                "description": "Natural language description of the test to generate"
                            },
                            "test_type": {
                                "type": "string",
                                "description": "Preferred test type (api, database, integration, auth)",
                                "enum": ["api", "database", "integration", "auth"],
                                "default": "api"
                            },
                            "environment": {
                                "type": "string", 
                                "description": "Target environment for the test",
                                "default": "staging"
                            },
                            "output_path": {
                                "type": "string",
                                "description": "Output path for generated test file",
                                "default": ".rocketship/generated-test.yaml"
                            },
                            "context": {
                                "type": "object",
                                "description": "Additional context from codebase analysis",
                                "properties": {
                                    "base_url": {"type": "string"},
                                    "auth_type": {"type": "string"},
                                    "database_type": {"type": "string"}
                                },
                                "additionalProperties": True
                            }
                        },
                        "required": ["prompt"]
                    }
                ),
                Tool(
                    name="validate_test_file",
                    description="Validate a Rocketship test file using the CLI",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "file_path": {
                                "type": "string",
                                "description": "Path to the Rocketship test file to validate"
                            }
                        },
                        "required": ["file_path"]
                    }
                ),
                Tool(
                    name="run_and_analyze_tests",
                    description="Execute Rocketship tests and provide intelligent analysis of results",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "file_path": {
                                "type": "string",
                                "description": "Path to the Rocketship test file to run"
                            },
                            "environment": {
                                "type": "string",
                                "description": "Environment to run tests against",
                                "default": "staging"
                            },
                            "var_file": {
                                "type": "string",
                                "description": "Path to variable file for the environment"
                            }
                        },
                        "required": ["file_path"]
                    }
                ),
                Tool(
                    name="analyze_git_diff",
                    description="Analyze git changes and suggest test updates",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "base_branch": {
                                "type": "string",
                                "description": "Base branch for comparison",
                                "default": "main"
                            },
                            "feature_branch": {
                                "type": "string", 
                                "description": "Feature branch to analyze",
                                "default": "HEAD"
                            },
                            "project_root": {
                                "type": "string",
                                "description": "Root directory of the git repository",
                                "default": "."
                            }
                        },
                        "required": []
                    }
                )
            ]
        
        @self.server.call_tool()
        async def handle_call_tool(name: str, arguments: dict) -> list[TextContent]:
            """Handle tool execution requests."""
            
            if name == "scan_and_generate_test_suite":
                return await self._handle_scan_and_generate(arguments)
            elif name == "generate_test_from_prompt":
                return await self._handle_generate_from_prompt(arguments)
            elif name == "validate_test_file":
                return await self._handle_validate_test(arguments)
            elif name == "run_and_analyze_tests":
                return await self._handle_run_tests(arguments)
            elif name == "analyze_git_diff":
                return await self._handle_git_diff(arguments)
            else:
                raise ValueError(f"Unknown tool: {name}")
    
    async def _handle_scan_and_generate(self, arguments: dict) -> list[TextContent]:
        """Handle scan_and_generate_test_suite tool."""
        
        project_root = arguments.get("project_root", ".")
        environments = arguments.get("environments", ["staging", "prod"])
        codebase_data = arguments.get("codebase_analysis", {})
        
        # Convert codebase data to CodebaseAnalysis object
        analysis = CodebaseAnalysis(
            api_endpoints=codebase_data.get("api_endpoints", []),
            database_schemas=codebase_data.get("database_schemas", []),
            service_configs=codebase_data.get("service_configs", []),
            environment_files=codebase_data.get("environment_files", []),
            test_suggestions=[]
        )
        
        # Generate test suite structure
        self.generator.project_root = project_root
        structure = self.generator.generate_test_suite_structure(analysis, environments)
        
        # Create .rocketship directory
        rocketship_dir = ensure_rocketship_directory(project_root)
        
        created_files = []
        
        try:
            # Create environment variable files
            for env_file, env_config in structure["environment_vars"].items():
                env_path = rocketship_dir / env_file
                save_yaml_file(env_config, str(env_path))
                created_files.append(str(env_path))
            
            # Create test directories and files
            for directory in structure["directories"]:
                dir_path = rocketship_dir / directory
                dir_path.mkdir(exist_ok=True)
            
            for file_path, file_content in structure["files"].items():
                full_path = rocketship_dir / file_path
                full_path.parent.mkdir(parents=True, exist_ok=True)
                save_yaml_file(file_content, str(full_path))
                created_files.append(str(full_path))
            
            # Generate summary
            summary = {
                "success": True,
                "structure_created": {
                    "directories": structure["directories"],
                    "environment_files": list(structure["environment_vars"].keys()),
                    "test_files": list(structure["files"].keys())
                },
                "created_files": created_files,
                "next_steps": [
                    "Review generated environment variable files and update with actual values",
                    "Customize test cases based on your specific API endpoints and requirements",
                    "Run 'rocketship validate' on the generated test files",
                    "Execute tests with: rocketship run -af .rocketship/[test-suite]/rocketship.yaml --var-file .rocketship/staging-vars.yaml"
                ]
            }
            
            return [TextContent(
                type="text",
                text=f"✅ Successfully generated Rocketship test suite structure!\n\n"
                     f"📁 Created directories: {', '.join(structure['directories'])}\n"
                     f"📄 Generated {len(created_files)} files\n"
                     f"🌍 Environment configs: {', '.join(structure['environment_vars'].keys())}\n\n"
                     f"Next steps:\n" + '\n'.join(f"• {step}" for step in summary['next_steps'])
            )]
        
        except Exception as e:
            return [TextContent(
                type="text",
                text=f"❌ Failed to generate test suite: {str(e)}\n\n"
                     f"Please ensure the project directory is writable and try again."
            )]
    
    async def _handle_generate_from_prompt(self, arguments: dict) -> list[TextContent]:
        """Handle generate_test_from_prompt tool."""
        
        prompt = arguments["prompt"]
        test_type = arguments.get("test_type", "api")
        environment = arguments.get("environment", "staging")
        output_path = arguments.get("output_path", ".rocketship/generated-test.yaml")
        context = arguments.get("context", {})
        
        try:
            # Generate test YAML
            test_yaml = self.generator.generate_test_from_prompt(prompt, context)
            
            # Ensure output directory exists
            output_file = Path(output_path)
            output_file.parent.mkdir(parents=True, exist_ok=True)
            
            # Save generated test
            with open(output_file, 'w') as f:
                f.write(test_yaml)
            
            return [TextContent(
                type="text",
                text=f"✅ Successfully generated test from prompt!\n\n"
                     f"📝 Prompt: {prompt}\n"
                     f"📁 Output: {output_path}\n"
                     f"🎯 Type: {test_type}\n"
                     f"🌍 Environment: {environment}\n\n"
                     f"Generated test file:\n```yaml\n{test_yaml}```\n\n"
                     f"Next steps:\n"
                     f"• Review and customize the generated test\n"
                     f"• Validate with: rocketship validate {output_path}\n"
                     f"• Run with: rocketship run -af {output_path}"
            )]
        
        except Exception as e:
            return [TextContent(
                type="text",
                text=f"❌ Failed to generate test from prompt: {str(e)}\n\n"
                     f"Please check the prompt and try again."
            )]
    
    async def _handle_validate_test(self, arguments: dict) -> list[TextContent]:
        """Handle validate_test_file tool."""
        
        file_path = arguments["file_path"]
        
        if not os.path.exists(file_path):
            return [TextContent(
                type="text",
                text=f"❌ Test file not found: {file_path}\n\n"
                     f"Please ensure the file exists and the path is correct."
            )]
        
        try:
            result = self.cli_wrapper.validate_test_file(file_path)
            
            if result.valid:
                return [TextContent(
                    type="text",
                    text=f"✅ Test file validation successful!\n\n"
                         f"📁 File: {file_path}\n"
                         f"✨ Status: Valid and ready to run\n\n"
                         f"Suggestions:\n" + '\n'.join(f"• {s}" for s in result.suggestions)
                )]
            else:
                error_text = '\n'.join(f"• {error}" for error in result.errors)
                suggestion_text = '\n'.join(f"• {s}" for s in result.suggestions)
                
                return [TextContent(
                    type="text",
                    text=f"❌ Test file validation failed!\n\n"
                         f"📁 File: {file_path}\n\n"
                         f"Errors:\n{error_text}\n\n"
                         f"Suggestions:\n{suggestion_text}"
                )]
        
        except Exception as e:
            return [TextContent(
                type="text",
                text=f"❌ Validation error: {str(e)}\n\n"
                     f"Please ensure Rocketship CLI is installed and accessible."
            )]
    
    async def _handle_run_tests(self, arguments: dict) -> list[TextContent]:
        """Handle run_and_analyze_tests tool."""
        
        file_path = arguments["file_path"]
        environment = arguments.get("environment", "staging")
        var_file = arguments.get("var_file")
        
        if not os.path.exists(file_path):
            return [TextContent(
                type="text",
                text=f"❌ Test file not found: {file_path}\n\n"
                     f"Please ensure the file exists and the path is correct."
            )]
        
        try:
            result = self.cli_wrapper.run_tests(file_path, environment, var_file)
            
            if result.success:
                return [TextContent(
                    type="text",
                    text=f"✅ Test execution successful!\n\n"
                         f"📁 File: {file_path}\n"
                         f"🌍 Environment: {environment}\n"
                         f"⏱️ Exit code: {result.exit_code}\n\n"
                         f"Output summary:\n{result.output[:500]}...\n\n"
                         f"Suggestions:\n" + '\n'.join(f"• {s}" for s in result.suggestions)
                )]
            else:
                suggestion_text = '\n'.join(f"• {s}" for s in result.suggestions)
                
                return [TextContent(
                    type="text",
                    text=f"❌ Test execution failed!\n\n"
                         f"📁 File: {file_path}\n"
                         f"🌍 Environment: {environment}\n"
                         f"⏱️ Exit code: {result.exit_code}\n\n"
                         f"Failure Analysis:\n{result.error_analysis}\n\n"
                         f"Output:\n{result.output[:1000]}...\n\n"
                         f"Suggestions:\n{suggestion_text}"
                )]
        
        except Exception as e:
            return [TextContent(
                type="text",
                text=f"❌ Test execution error: {str(e)}\n\n"
                     f"Please ensure Rocketship CLI is installed and the test file is valid."
            )]
    
    async def _handle_git_diff(self, arguments: dict) -> list[TextContent]:
        """Handle analyze_git_diff tool."""
        
        base_branch = arguments.get("base_branch", "main")
        feature_branch = arguments.get("feature_branch", "HEAD")
        project_root = arguments.get("project_root", ".")
        
        try:
            self.git_analyzer.repo_path = project_root
            analysis = self.git_analyzer.analyze_diff(base_branch, feature_branch)
            
            if "error" in analysis:
                return [TextContent(
                    type="text",
                    text=f"❌ Git analysis failed: {analysis['error']}\n\n"
                         f"Please ensure you're in a git repository and the branches exist."
                )]
            
            file_changes = analysis["file_changes"]
            suggestions = analysis["test_suggestions"]
            confidence = analysis["confidence_level"]
            
            # Format file changes summary
            changes_by_type = {}
            for change in file_changes:
                change_type = change["change_type"]
                if change_type not in changes_by_type:
                    changes_by_type[change_type] = []
                changes_by_type[change_type].append(change["filename"])
            
            changes_summary = []
            for change_type, files in changes_by_type.items():
                changes_summary.append(f"• {change_type.title()}: {len(files)} files")
                for file in files[:3]:  # Show first 3 files
                    changes_summary.append(f"  - {file}")
                if len(files) > 3:
                    changes_summary.append(f"  - ... and {len(files) - 3} more")
            
            suggestion_text = '\n'.join(f"• {s}" for s in suggestions)
            changes_text = '\n'.join(changes_summary)
            
            confidence_emoji = {"high": "🔴", "medium": "🟡", "low": "🟢"}
            
            return [TextContent(
                type="text",
                text=f"📊 Git Diff Analysis Complete!\n\n"
                     f"🔀 Comparing: {base_branch}...{feature_branch}\n"
                     f"📁 Repository: {project_root}\n"
                     f"{confidence_emoji.get(confidence, '⚪')} Confidence: {confidence}\n\n"
                     f"File Changes:\n{changes_text}\n\n"
                     f"Test Suggestions:\n{suggestion_text}\n\n"
                     f"💡 These suggestions are based on file change patterns. "
                     f"Human review is recommended before implementing changes."
            )]
        
        except Exception as e:
            return [TextContent(
                type="text",
                text=f"❌ Git analysis error: {str(e)}\n\n"
                     f"Please ensure you're in a git repository with the specified branches."
            )]


def main():
    """Main entry point for the MCP server."""
    
    server_instance = RocketshipMCPServer()
    
    async def run_server():
        """Run the MCP server."""
        async with stdio_server() as (read_stream, write_stream):
            await server_instance.server.run(
                read_stream,
                write_stream,
                InitializationOptions(
                    server_name="rocketship",
                    server_version="0.1.0",
                    capabilities=server_instance.server.get_capabilities(
                        notification_options=None,
                        experimental_capabilities=None
                    )
                )
            )
    
    # Run the server
    asyncio.run(run_server())


if __name__ == "__main__":
    main()