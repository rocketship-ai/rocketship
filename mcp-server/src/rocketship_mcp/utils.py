"""Utility functions for Rocketship MCP server."""

import os
import subprocess
import yaml
from typing import Dict, List, Optional, Any
from pathlib import Path

from .types import ValidationResult, TestExecutionResult


class RocketshipCLIWrapper:
    """Wrapper for Rocketship CLI operations."""
    
    def __init__(self, cli_path: str = "rocketship"):
        self.cli_path = cli_path
    
    def validate_test_file(self, file_path: str) -> ValidationResult:
        """Validate a Rocketship test file using the CLI."""
        
        try:
            result = subprocess.run(
                [self.cli_path, "validate", file_path],
                capture_output=True,
                text=True,
                timeout=30
            )
            
            if result.returncode == 0:
                return ValidationResult(
                    valid=True,
                    errors=[],
                    warnings=[],
                    suggestions=["Test file is valid and ready to run"]
                )
            else:
                # Parse validation errors from output
                errors = self._parse_validation_errors(result.stderr)
                return ValidationResult(
                    valid=False,
                    errors=errors,
                    warnings=[],
                    suggestions=self._generate_validation_suggestions(errors)
                )
        
        except subprocess.TimeoutExpired:
            return ValidationResult(
                valid=False,
                errors=["Validation timed out after 30 seconds"],
                warnings=[],
                suggestions=["Check if test file is too large or complex"]
            )
        
        except Exception as e:
            return ValidationResult(
                valid=False,
                errors=[f"Validation failed: {str(e)}"],
                warnings=[],
                suggestions=["Ensure Rocketship CLI is installed and accessible"]
            )
    
    def run_tests(
        self, 
        file_path: str, 
        environment: str = "staging",
        var_file: Optional[str] = None
    ) -> TestExecutionResult:
        """Run Rocketship tests and analyze results."""
        
        cmd = [self.cli_path, "run", "-af", file_path]
        
        if var_file:
            cmd.extend(["--var-file", var_file])
        
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300  # 5 minutes
            )
            
            success = result.returncode == 0
            output = result.stdout + result.stderr
            
            error_analysis = None
            suggestions = []
            
            if not success:
                error_analysis = self._analyze_test_failures(output)
                suggestions = self._generate_failure_suggestions(output)
            else:
                suggestions = ["All tests passed successfully!"]
            
            return TestExecutionResult(
                success=success,
                exit_code=result.returncode,
                output=output,
                error_analysis=error_analysis,
                suggestions=suggestions
            )
        
        except subprocess.TimeoutExpired:
            return TestExecutionResult(
                success=False,
                exit_code=-1,
                output="Test execution timed out after 5 minutes",
                error_analysis="Tests took too long to complete",
                suggestions=[
                    "Consider reducing test scope or increasing timeouts",
                    "Check for infinite loops or hanging operations"
                ]
            )
        
        except Exception as e:
            return TestExecutionResult(
                success=False,
                exit_code=-1,
                output=f"Execution failed: {str(e)}",
                error_analysis=str(e),
                suggestions=["Ensure Rocketship CLI is installed and test file exists"]
            )
    
    def _parse_validation_errors(self, stderr: str) -> List[str]:
        """Parse validation errors from CLI output."""
        
        errors = []
        lines = stderr.split('\n')
        
        for line in lines:
            line = line.strip()
            if line and not line.startswith('['):  # Skip log prefixes
                errors.append(line)
        
        return errors if errors else ["Unknown validation error"]
    
    def _generate_validation_suggestions(self, errors: List[str]) -> List[str]:
        """Generate suggestions based on validation errors."""
        
        suggestions = []
        
        for error in errors:
            error_lower = error.lower()
            
            if "yaml" in error_lower or "syntax" in error_lower:
                suggestions.append("Check YAML syntax - ensure proper indentation and structure")
            elif "plugin" in error_lower:
                suggestions.append("Verify plugin name is correct and supported")
            elif "required" in error_lower:
                suggestions.append("Add missing required fields to configuration")
            elif "assertion" in error_lower:
                suggestions.append("Review assertion syntax and expected values")
            else:
                suggestions.append("Review Rocketship documentation for proper configuration")
        
        return suggestions if suggestions else ["Check test file format and syntax"]
    
    def _analyze_test_failures(self, output: str) -> str:
        """Analyze test execution output for failure patterns."""
        
        failure_indicators = [
            ("connection refused", "Service appears to be down or unreachable"),
            ("timeout", "Request timed out - service may be slow or overloaded"),
            ("401", "Authentication failed - check API keys and tokens"),
            ("403", "Permission denied - check access privileges"),
            ("404", "Resource not found - verify URLs and endpoints"),
            ("500", "Server error - check service logs for details"),
            ("assertion failed", "Response data doesn't match expected values"),
            ("json parse", "Invalid JSON response - check API response format"),
            ("dns", "DNS resolution failed - check network connectivity")
        ]
        
        analysis = []
        output_lower = output.lower()
        
        for pattern, explanation in failure_indicators:
            if pattern in output_lower:
                analysis.append(explanation)
        
        if not analysis:
            analysis.append("Test failed for unknown reasons - check full output for details")
        
        return "; ".join(analysis)
    
    def _generate_failure_suggestions(self, output: str) -> List[str]:
        """Generate suggestions based on test failures."""
        
        suggestions = []
        output_lower = output.lower()
        
        if "connection refused" in output_lower:
            suggestions.extend([
                "Verify the service is running and accessible",
                "Check network connectivity and firewall settings",
                "Confirm the correct host and port configuration"
            ])
        
        if "timeout" in output_lower:
            suggestions.extend([
                "Increase timeout values in test configuration",
                "Check if the service is experiencing high load",
                "Verify network latency between test runner and service"
            ])
        
        if any(code in output_lower for code in ["401", "403"]):
            suggestions.extend([
                "Verify API keys and authentication tokens",
                "Check if credentials have expired",
                "Ensure proper authorization headers are set"
            ])
        
        if "404" in output_lower:
            suggestions.extend([
                "Verify API endpoints and URLs are correct",
                "Check if the service version matches test expectations",
                "Confirm the resource exists in the target environment"
            ])
        
        if "assertion failed" in output_lower:
            suggestions.extend([
                "Review expected values in test assertions",
                "Check if API response format has changed",
                "Update test data to match current service behavior"
            ])
        
        if not suggestions:
            suggestions.append("Review test configuration and service status")
        
        return suggestions


class GitAnalyzer:
    """Analyze git changes for test suggestions."""
    
    def __init__(self, repo_path: str = "."):
        self.repo_path = Path(repo_path)
    
    def analyze_diff(
        self, 
        base_branch: str = "main",
        feature_branch: str = "HEAD"
    ) -> Dict[str, Any]:
        """Analyze git diff to suggest test updates."""
        
        try:
            # Get list of changed files
            result = subprocess.run(
                ["git", "diff", "--name-status", f"{base_branch}...{feature_branch}"],
                capture_output=True,
                text=True,
                cwd=self.repo_path
            )
            
            if result.returncode != 0:
                return {
                    "error": "Failed to get git diff",
                    "file_changes": [],
                    "test_suggestions": []
                }
            
            file_changes = self._parse_file_changes(result.stdout)
            test_suggestions = self._generate_test_suggestions(file_changes)
            
            return {
                "file_changes": file_changes,
                "test_suggestions": test_suggestions,
                "confidence_level": self._assess_confidence(file_changes)
            }
        
        except Exception as e:
            return {
                "error": f"Git analysis failed: {str(e)}",
                "file_changes": [],
                "test_suggestions": []
            }
    
    def _parse_file_changes(self, diff_output: str) -> List[Dict[str, Any]]:
        """Parse git diff output into structured file changes."""
        
        changes = []
        lines = diff_output.strip().split('\n')
        
        for line in lines:
            if not line:
                continue
            
            parts = line.split('\t')
            if len(parts) >= 2:
                status = parts[0]
                filename = parts[1]
                
                change_type = self._classify_change_type(status)
                file_type = self._classify_file_type(filename)
                
                changes.append({
                    "status": status,
                    "filename": filename,
                    "change_type": change_type,
                    "file_type": file_type,
                    "test_impact": self._assess_test_impact(filename, status)
                })
        
        return changes
    
    def _classify_change_type(self, status: str) -> str:
        """Classify git change status."""
        
        if status.startswith('A'):
            return "added"
        elif status.startswith('M'):
            return "modified"
        elif status.startswith('D'):
            return "deleted"
        elif status.startswith('R'):
            return "renamed"
        else:
            return "unknown"
    
    def _classify_file_type(self, filename: str) -> str:
        """Classify file type for test impact assessment."""
        
        path = Path(filename)
        
        # API routes and controllers
        if any(keyword in filename.lower() for keyword in ['route', 'controller', 'handler', 'endpoint']):
            return "api"
        
        # Database related
        if any(keyword in filename.lower() for keyword in ['migration', 'model', 'schema', 'sql']):
            return "database"
        
        # Configuration files
        if any(ext in path.suffix.lower() for ext in ['.env', '.yaml', '.yml', '.json', '.toml']):
            return "config"
        
        # Documentation
        if path.suffix.lower() in ['.md', '.rst', '.txt']:
            return "docs"
        
        # Test files
        if any(keyword in filename.lower() for keyword in ['test', 'spec']):
            return "test"
        
        # Code files
        if path.suffix.lower() in ['.py', '.js', '.ts', '.go', '.java', '.rb', '.php']:
            return "code"
        
        return "other"
    
    def _assess_test_impact(self, filename: str, status: str) -> str:
        """Assess how file changes impact testing needs."""
        
        file_type = self._classify_file_type(filename)
        
        if file_type == "api":
            if status.startswith('A'):
                return "new_tests_needed"
            elif status.startswith('M'):
                return "update_tests"
            elif status.startswith('D'):
                return "remove_tests"
        
        elif file_type == "database":
            if status.startswith('A') or status.startswith('M'):
                return "update_schema_tests"
            elif status.startswith('D'):
                return "remove_schema_tests"
        
        elif file_type == "config":
            return "update_config_tests"
        
        elif file_type == "test":
            return "review_test_changes"
        
        return "low_impact"
    
    def _generate_test_suggestions(self, file_changes: List[Dict[str, Any]]) -> List[str]:
        """Generate test suggestions based on file changes."""
        
        suggestions = []
        
        # Group changes by impact type
        impacts = {}
        for change in file_changes:
            impact = change["test_impact"]
            if impact not in impacts:
                impacts[impact] = []
            impacts[impact].append(change)
        
        # Generate suggestions for each impact type
        if "new_tests_needed" in impacts:
            api_files = [c["filename"] for c in impacts["new_tests_needed"]]
            suggestions.append(
                f"Add integration tests for new API endpoints: {', '.join(api_files[:3])}"
            )
        
        if "update_tests" in impacts:
            modified_files = [c["filename"] for c in impacts["update_tests"]]
            suggestions.append(
                f"Update existing tests for modified endpoints: {', '.join(modified_files[:3])}"
            )
        
        if "update_schema_tests" in impacts:
            suggestions.append("Update database/schema tests for migration changes")
        
        if "update_config_tests" in impacts:
            suggestions.append("Review and update configuration-related tests")
        
        if "remove_tests" in impacts:
            removed_files = [c["filename"] for c in impacts["remove_tests"]]
            suggestions.append(
                f"Remove tests for deleted components: {', '.join(removed_files[:3])}"
            )
        
        # Add general suggestions if no specific impacts found
        if not suggestions:
            if any(c["file_type"] == "code" for c in file_changes):
                suggestions.append("Consider adding tests for modified code components")
            else:
                suggestions.append("No immediate test changes required for these file modifications")
        
        return suggestions
    
    def _assess_confidence(self, file_changes: List[Dict[str, Any]]) -> str:
        """Assess confidence level in test suggestions."""
        
        high_impact_count = len([
            c for c in file_changes 
            if c["test_impact"] in ["new_tests_needed", "update_tests", "update_schema_tests"]
        ])
        
        if high_impact_count >= 3:
            return "high"
        elif high_impact_count >= 1:
            return "medium"
        else:
            return "low"


def load_yaml_file(file_path: str) -> Dict[str, Any]:
    """Load and parse a YAML file."""
    
    try:
        with open(file_path, 'r') as f:
            return yaml.safe_load(f)
    except Exception as e:
        raise ValueError(f"Failed to load YAML file {file_path}: {str(e)}")


def save_yaml_file(data: Dict[str, Any], file_path: str) -> None:
    """Save data to a YAML file."""
    
    os.makedirs(os.path.dirname(file_path), exist_ok=True)
    
    with open(file_path, 'w') as f:
        yaml.dump(data, f, default_flow_style=False, sort_keys=False)


def ensure_rocketship_directory(project_root: str = ".") -> Path:
    """Ensure .rocketship directory exists and return its path."""
    
    rocketship_dir = Path(project_root) / ".rocketship"
    rocketship_dir.mkdir(exist_ok=True)
    
    return rocketship_dir