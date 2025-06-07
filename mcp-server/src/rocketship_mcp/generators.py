"""Test generation utilities for Rocketship MCP server."""

import re
import yaml
from typing import Dict, List, Any
from pathlib import Path

from .types import CodebaseAnalysis


class RocketshipTestGenerator:
    """Generate Rocketship test configurations and structures."""
    
    ROCKETSHIP_PLUGINS = [
        "http", "sql", "supabase", "delay", "log", 
        "script", "agent", "browser"
    ]
    
    def __init__(self, project_root: str = "."):
        self.project_root = Path(project_root)
        self.rocketship_dir = self.project_root / ".rocketship"
    
    def generate_test_suite_structure(
        self, 
        analysis: CodebaseAnalysis,
        environments: List[str] = None
    ) -> Dict[str, Any]:
        """Generate complete .rocketship directory structure."""
        
        if environments is None:
            environments = ["staging", "prod"]
        
        structure = {
            "directories": [],
            "files": {},
            "environment_vars": {}
        }
        
        # Generate environment variable files
        for env in environments:
            env_config = self._generate_environment_config(analysis, env)
            structure["environment_vars"][f"{env}-vars.yaml"] = env_config
        
        # Generate test suites based on analysis
        if analysis.api_endpoints:
            structure["directories"].append("api-tests")
            structure["files"]["api-tests/rocketship.yaml"] = self._generate_api_test_suite(analysis)
        
        if analysis.database_schemas:
            structure["directories"].append("database-tests")
            structure["files"]["database-tests/rocketship.yaml"] = self._generate_database_test_suite(analysis)
        
        # Always include integration tests
        structure["directories"].append("integration-tests")
        structure["files"]["integration-tests/rocketship.yaml"] = self._generate_integration_test_suite(analysis)
        
        return structure
    
    def generate_test_from_prompt(
        self, 
        prompt: str, 
        context: Dict[str, Any] = None
    ) -> str:
        """Generate a Rocketship test YAML from natural language prompt."""
        
        if context is None:
            context = {}
        
        # Analyze prompt to determine test type and structure
        test_type = self._infer_test_type(prompt)
        plugin = self._infer_plugin(prompt, test_type)
        
        # Generate base test structure
        test_config = {
            "name": self._generate_test_name(prompt),
            "description": f"Generated from prompt: {prompt}",
            "vars": self._generate_default_vars(context),
            "tests": [
                {
                    "name": self._generate_test_case_name(prompt),
                    "steps": self._generate_test_steps(prompt, plugin, context)
                }
            ]
        }
        
        return yaml.dump(test_config, default_flow_style=False, sort_keys=False)
    
    def _generate_environment_config(
        self, 
        analysis: CodebaseAnalysis, 
        environment: str
    ) -> Dict[str, Any]:
        """Generate environment-specific variable configuration."""
        
        config = {
            "# Environment-specific variables": None,
            f"# Generated for {environment} environment": None,
            "base_url": f"https://api-{environment}.example.com",
            "timeout": 30,
        }
        
        # Add database configuration if detected
        if analysis.database_schemas:
            config.update({
                "database": {
                    "host": f"{{ .env.DB_HOST_{environment.upper()} }}",
                    "user": f"{{ .env.DB_USER_{environment.upper()} }}",
                    "password": f"{{ .env.DB_PASSWORD_{environment.upper()} }}",
                    "name": f"{{ .env.DB_NAME_{environment.upper()} }}"
                }
            })
        
        # Add auth configuration
        config.update({
            "auth": {
                "api_key": f"{{ .env.API_KEY_{environment.upper()} }}",
                "bearer_token": f"{{ .env.BEARER_TOKEN_{environment.upper()} }}"
            }
        })
        
        # Add service-specific configurations based on analysis
        for service in analysis.service_configs:
            service_name = service.get("name", "service")
            config[f"{service_name}_config"] = {
                "url": f"{{ .env.{service_name.upper()}_URL_{environment.upper()} }}",
                "key": f"{{ .env.{service_name.upper()}_KEY_{environment.upper()} }}"
            }
        
        return config
    
    def _generate_api_test_suite(self, analysis: CodebaseAnalysis) -> Dict[str, Any]:
        """Generate API test suite configuration."""
        
        return {
            "name": "API Integration Tests",
            "description": "Comprehensive API endpoint testing",
            "vars": {
                "api_base": "{{ .vars.base_url }}",
                "auth_header": "{{ .vars.auth.api_key }}"
            },
            "tests": [
                {
                    "name": "API Health Check",
                    "steps": [
                        {
                            "name": "Health endpoint",
                            "plugin": "http",
                            "config": {
                                "method": "GET",
                                "url": "{{ .vars.api_base }}/health",
                                "timeout": "{{ .vars.timeout }}s"
                            },
                            "assertions": [
                                {
                                    "type": "status_code",
                                    "expected": 200
                                }
                            ]
                        }
                    ]
                }
            ] + self._generate_endpoint_tests(analysis.api_endpoints)
        }
    
    def _generate_database_test_suite(self, analysis: CodebaseAnalysis) -> Dict[str, Any]:
        """Generate database test suite configuration."""
        
        return {
            "name": "Database Tests",
            "description": "Database schema and data validation tests",
            "vars": {
                "db_dsn": "postgres://{{ .vars.database.user }}:{{ .vars.database.password }}@{{ .vars.database.host }}/{{ .vars.database.name }}?sslmode=disable"
            },
            "tests": [
                {
                    "name": "Database Connectivity",
                    "steps": [
                        {
                            "name": "Test connection",
                            "plugin": "sql",
                            "config": {
                                "driver": "postgres",
                                "dsn": "{{ .vars.db_dsn }}",
                                "commands": ["SELECT 1 as test_connection;"]
                            },
                            "assertions": [
                                {
                                    "type": "row_count",
                                    "query_index": 0,
                                    "expected": 1
                                }
                            ]
                        }
                    ]
                }
            ] + self._generate_schema_tests(analysis.database_schemas)
        }
    
    def _generate_integration_test_suite(self, analysis: CodebaseAnalysis) -> Dict[str, Any]:
        """Generate integration test suite configuration."""
        
        return {
            "name": "Integration Tests",
            "description": "End-to-end workflow testing",
            "vars": {
                "api_base": "{{ .vars.base_url }}",
                "auth_token": "{{ .vars.auth.bearer_token }}"
            },
            "tests": [
                {
                    "name": "Basic Integration Flow",
                    "steps": [
                        {
                            "name": "System health check",
                            "plugin": "http",
                            "config": {
                                "method": "GET",
                                "url": "{{ .vars.api_base }}/health"
                            },
                            "assertions": [
                                {
                                    "type": "status_code",
                                    "expected": 200
                                }
                            ]
                        },
                        {
                            "name": "Log test completion",
                            "plugin": "log",
                            "config": {
                                "message": "✅ Integration test completed successfully"
                            }
                        }
                    ]
                }
            ]
        }
    
    def _generate_endpoint_tests(self, endpoints: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Generate test cases for detected API endpoints."""
        
        tests = []
        
        for endpoint in endpoints[:3]:  # Limit to first 3 endpoints
            method = endpoint.get("method", "GET").upper()
            path = endpoint.get("path", "/")
            
            test_name = f"{method} {path}".replace("/", "_").replace("{", "").replace("}", "")
            
            step_config = {
                "method": method,
                "url": f"{{{{ .vars.api_base }}}}{path}",
                "headers": {
                    "Authorization": "Bearer {{ .vars.auth_token }}"
                }
            }
            
            # Add body for POST/PUT/PATCH
            if method in ["POST", "PUT", "PATCH"]:
                step_config["body"] = {"example": "data"}
            
            tests.append({
                "name": f"Test {test_name}",
                "steps": [
                    {
                        "name": f"{method} request to {path}",
                        "plugin": "http",
                        "config": step_config,
                        "assertions": [
                            {
                                "type": "status_code", 
                                "expected": 200 if method == "GET" else 201
                            }
                        ]
                    }
                ]
            })
        
        return tests
    
    def _generate_schema_tests(self, schemas: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Generate test cases for database schemas."""
        
        tests = []
        
        for schema in schemas[:2]:  # Limit to first 2 schemas
            table_name = schema.get("table", "test_table")
            
            tests.append({
                "name": f"Test {table_name} schema",
                "steps": [
                    {
                        "name": f"Query {table_name}",
                        "plugin": "sql",
                        "config": {
                            "driver": "postgres",
                            "dsn": "{{ .vars.db_dsn }}",
                            "commands": [f"SELECT COUNT(*) as count FROM {table_name};"]
                        },
                        "assertions": [
                            {
                                "type": "success_count",
                                "expected": 1
                            }
                        ]
                    }
                ]
            })
        
        return tests
    
    def _infer_test_type(self, prompt: str) -> str:
        """Infer test type from prompt text."""
        
        prompt_lower = prompt.lower()
        
        if any(keyword in prompt_lower for keyword in ["api", "endpoint", "rest", "http"]):
            return "api"
        elif any(keyword in prompt_lower for keyword in ["database", "sql", "table", "query"]):
            return "database"
        elif any(keyword in prompt_lower for keyword in ["auth", "login", "signup", "user"]):
            return "auth"
        elif any(keyword in prompt_lower for keyword in ["integration", "e2e", "workflow"]):
            return "integration"
        else:
            return "api"  # Default to API tests
    
    def _infer_plugin(self, prompt: str, test_type: str) -> str:
        """Infer which Rocketship plugin to use."""
        
        prompt_lower = prompt.lower()
        
        # Check for specific plugin mentions
        for plugin in self.ROCKETSHIP_PLUGINS:
            if plugin in prompt_lower:
                return plugin
        
        # Infer based on test type
        if test_type == "api":
            return "http"
        elif test_type == "database":
            if "supabase" in prompt_lower:
                return "supabase"
            else:
                return "sql"
        elif test_type == "auth":
            return "supabase" if "supabase" in prompt_lower else "http"
        else:
            return "http"
    
    def _generate_test_name(self, prompt: str) -> str:
        """Generate test name from prompt."""
        
        # Extract key words and create readable name
        words = re.findall(r'\w+', prompt.lower())
        key_words = [w for w in words if len(w) > 2 and w not in ['test', 'testing', 'create', 'generate']]
        
        if not key_words:
            return "Generated Test Suite"
        
        return " ".join(key_words[:4]).title() + " Tests"
    
    def _generate_test_case_name(self, prompt: str) -> str:
        """Generate test case name from prompt."""
        
        return f"Test {self._generate_test_name(prompt).replace(' Tests', '')}"
    
    def _generate_default_vars(self, context: Dict[str, Any]) -> Dict[str, Any]:
        """Generate default variables for test."""
        
        return {
            "base_url": context.get("base_url", "{{ .vars.base_url }}"),
            "timeout": context.get("timeout", 30),
            "auth_token": "{{ .vars.auth.bearer_token }}"
        }
    
    def _generate_test_steps(
        self, 
        prompt: str, 
        plugin: str, 
        context: Dict[str, Any]
    ) -> List[Dict[str, Any]]:
        """Generate test steps based on prompt and plugin."""
        
        steps = []
        
        if plugin == "http":
            steps.append({
                "name": "HTTP request",
                "plugin": "http",
                "config": {
                    "method": "GET",
                    "url": "{{ .vars.base_url }}/endpoint",
                    "headers": {
                        "Authorization": "Bearer {{ .vars.auth_token }}"
                    }
                },
                "assertions": [
                    {
                        "type": "status_code",
                        "expected": 200
                    }
                ]
            })
        
        elif plugin == "sql":
            steps.append({
                "name": "Database query",
                "plugin": "sql", 
                "config": {
                    "driver": "postgres",
                    "dsn": "{{ .vars.db_dsn }}",
                    "commands": ["SELECT 1 as test;"]
                },
                "assertions": [
                    {
                        "type": "row_count",
                        "query_index": 0,
                        "expected": 1
                    }
                ]
            })
        
        elif plugin == "supabase":
            steps.append({
                "name": "Supabase operation",
                "plugin": "supabase",
                "config": {
                    "url": "{{ .vars.supabase_url }}",
                    "key": "{{ .vars.supabase_anon_key }}",
                    "operation": "select",
                    "table": "test_table",
                    "select": {
                        "columns": ["id", "name"],
                        "limit": 10
                    }
                },
                "assertions": [
                    {
                        "type": "json_path",
                        "path": "length",
                        "expected": "exists"
                    }
                ]
            })
        
        else:
            # Default to simple log step
            steps.append({
                "name": "Log message",
                "plugin": "log",
                "config": {
                    "message": f"Generated test step for: {prompt}"
                }
            })
        
        return steps