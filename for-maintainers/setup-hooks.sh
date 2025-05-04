#!/bin/bash

# Create the pre-commit hook
cat > .git/hooks/pre-commit << 'EOL'
#!/bin/sh

echo "Running lint and tests..."
if ! make lint test; then
    echo "❌ Lint or tests failed. Commit aborted."
    exit 1
fi

echo "✅ Lint and tests passed!"
exit 0
EOL

# Make it executable
chmod +x .git/hooks/pre-commit

echo "✅ Pre-commit hook installed successfully!" 
