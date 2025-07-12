#!/bin/bash

# Create the pre-commit hook
cat > .git/hooks/pre-commit << 'EOL'
#!/bin/sh

echo "Running lint and tests..."
if ! make lint test; then
    echo "❌ Lint or tests failed. Commit aborted."
    exit 1
fi

# Check if schema.json was modified
if git diff --cached --name-only | grep -q "internal/dsl/schema.json"; then
    echo "📚 Schema changed, regenerating plugin reference docs..."
    cd docs && python3 src/yaml-reference/generate-plugin-reference.py
    if [ $? -ne 0 ]; then
        echo "❌ Failed to generate plugin reference docs. Commit aborted."
        exit 1
    fi
    cd ..
    # Stage the updated docs
    git add docs/src/yaml-reference/plugin-reference.md
    echo "✅ Plugin reference docs updated!"
fi

echo "✅ Lint and tests passed!"
exit 0
EOL

# Make it executable
chmod +x .git/hooks/pre-commit

echo "✅ Pre-commit hook installed successfully!"

# Run initial dev setup if not already done
if [ ! -f "internal/embedded/bin/engine" ] || [ ! -f "internal/embedded/bin/worker" ]; then
    echo "Running initial development setup..."
    make build-binaries
fi 
