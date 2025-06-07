#!/usr/bin/env python3
"""Simple test script to verify MCP server imports and basic functionality."""

import sys
import os

# Add src to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

def test_imports():
    """Test that all modules can be imported successfully."""
    try:
        import rocketship_mcp
        print(f"✅ Successfully imported rocketship_mcp version {rocketship_mcp.__version__}")
        
        import rocketship_mcp.types
        print("✅ Successfully imported types")
        
        import rocketship_mcp.generators
        print("✅ Successfully imported generators")
        
        import rocketship_mcp.utils
        print("✅ Successfully imported utils")
        
        import rocketship_mcp.server
        print("✅ Successfully imported server")
        
        return True
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False

def test_basic_functionality():
    """Test basic functionality without external dependencies."""
    try:
        from rocketship_mcp.generators import RocketshipTestGenerator
        from rocketship_mcp.types import CodebaseAnalysis
        
        generator = RocketshipTestGenerator()
        
        # Test simple analysis
        analysis = CodebaseAnalysis(
            api_endpoints=[{"method": "GET", "path": "/test"}],
            database_schemas=[{"table": "test", "columns": ["id"]}]
        )
        
        # Test test generation from prompt
        test_yaml = generator.generate_test_from_prompt("Test API endpoint")
        assert "name:" in test_yaml
        assert "tests:" in test_yaml
        print("✅ Test generation from prompt works")
        
        # Test structure generation
        structure = generator.generate_test_suite_structure(analysis, ["staging"])
        assert "directories" in structure
        assert "files" in structure
        assert "environment_vars" in structure
        print("✅ Test suite structure generation works")
        
        return True
    except Exception as e:
        print(f"❌ Functionality test error: {e}")
        return False

if __name__ == "__main__":
    print("🚀 Testing Rocketship MCP Server...")
    print()
    
    success = True
    
    print("📦 Testing imports...")
    success &= test_imports()
    print()
    
    print("⚙️  Testing basic functionality...")
    success &= test_basic_functionality()
    print()
    
    if success:
        print("✅ All tests passed! MCP server is ready to use.")
        sys.exit(0)
    else:
        print("❌ Some tests failed. Please check the errors above.")
        sys.exit(1)