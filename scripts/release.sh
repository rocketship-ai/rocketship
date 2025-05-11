#!/bin/bash

# Check if a version was provided
if [ -z "$1" ]; then
    echo "Error: No version tag provided"
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.1.0"
    exit 1
fi

VERSION=$1

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in the format v0.0.0"
    echo "Example: v0.1.0"
    exit 1
fi

# Check if we're in the root directory
if [ ! -f "go.mod" ]; then
    echo "Error: Must be run from repository root"
    exit 1
fi

# Create and push the tag
echo "Creating tag $VERSION..."
git tag -a $VERSION -m "Release $VERSION"
git push origin $VERSION

echo "Tag $VERSION created and pushed!"
echo "GitHub Actions will now build and create the release."
echo "You can monitor the progress at: https://github.com/rocketship-ai/rocketship/actions" 