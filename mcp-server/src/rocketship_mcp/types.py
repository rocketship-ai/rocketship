"""Type definitions for Rocketship MCP server."""

from typing import Dict, List, Optional, Any
from pydantic import BaseModel, Field


class TestSuiteConfig(BaseModel):
    """Configuration for generating test suites."""
    
    name: str = Field(description="Name of the test suite")
    description: Optional[str] = Field(default=None, description="Test suite description")
    plugins: List[str] = Field(default_factory=list, description="Required plugins")
    environment_vars: Dict[str, Any] = Field(default_factory=dict, description="Environment variables")


class EnvironmentConfig(BaseModel):
    """Environment-specific configuration."""
    
    name: str = Field(description="Environment name (e.g., staging, prod)")
    base_url: Optional[str] = Field(default=None, description="API base URL")
    database_url: Optional[str] = Field(default=None, description="Database connection URL")
    auth_config: Dict[str, str] = Field(default_factory=dict, description="Authentication configuration")
    custom_vars: Dict[str, Any] = Field(default_factory=dict, description="Custom environment variables")


class CodebaseAnalysis(BaseModel):
    """Results from codebase analysis."""
    
    api_endpoints: List[Dict[str, Any]] = Field(default_factory=list, description="Detected API endpoints")
    database_schemas: List[Dict[str, Any]] = Field(default_factory=list, description="Database schema information")
    service_configs: List[Dict[str, Any]] = Field(default_factory=list, description="Service configurations")
    environment_files: List[str] = Field(default_factory=list, description="Environment configuration files")
    test_suggestions: List[str] = Field(default_factory=list, description="Suggested test types")


class TestGenerationRequest(BaseModel):
    """Request for test generation from prompt."""
    
    prompt: str = Field(description="Natural language test description")
    test_type: Optional[str] = Field(default=None, description="Preferred test type (api, db, integration)")
    context: Dict[str, Any] = Field(default_factory=dict, description="Additional context")
    environment: str = Field(default="staging", description="Target environment")


class GitDiffAnalysis(BaseModel):
    """Analysis of git diff for test suggestions."""
    
    base_branch: str = Field(default="main", description="Base branch for comparison")
    feature_branch: str = Field(default="HEAD", description="Feature branch to analyze")
    file_changes: List[Dict[str, Any]] = Field(default_factory=list, description="Changed files analysis")
    test_suggestions: List[str] = Field(default_factory=list, description="Suggested test updates")
    confidence_level: str = Field(default="medium", description="Confidence in suggestions")


class TestExecutionResult(BaseModel):
    """Results from test execution."""
    
    success: bool = Field(description="Whether tests passed")
    exit_code: int = Field(description="CLI exit code")
    output: str = Field(description="Test execution output")
    error_analysis: Optional[str] = Field(default=None, description="AI analysis of failures")
    suggestions: List[str] = Field(default_factory=list, description="Improvement suggestions")


class ValidationResult(BaseModel):
    """Results from test validation."""
    
    valid: bool = Field(description="Whether test file is valid")
    errors: List[str] = Field(default_factory=list, description="Validation errors")
    warnings: List[str] = Field(default_factory=list, description="Validation warnings")
    suggestions: List[str] = Field(default_factory=list, description="Improvement suggestions")