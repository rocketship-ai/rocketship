# Environment Variables Example

This example demonstrates how to use environment variables in Rocketship tests using the `--env-file` flag.

## Setup

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` and update the values for your environment:
```bash
# At minimum, update these values:
API_BASE_URL=https://tryme.rocketship.sh
API_TOKEN=your-actual-token
API_KEY=your-actual-key
```

## Running the Example

### Using --env-file flag (Recommended)
```bash
# Load environment variables from .env file
rocketship run -af rocketship.yaml --env-file .env
```

### Using system environment variables
```bash
# Export variables manually
export API_BASE_URL=https://tryme.rocketship.sh
export API_TOKEN=your-token
export API_KEY=your-key

# Run tests
rocketship run -af rocketship.yaml
```

### For CI/CD environments
```bash
# GitHub Actions example - secrets are already in environment
rocketship run -af rocketship.yaml
```

## Features Demonstrated

1. **Loading from .env file** - Keep sensitive data out of version control
2. **Environment variable access** - Use `{{ .env.VARIABLE_NAME }}` syntax
3. **Mixed with runtime variables** - Combine env vars with test execution data
4. **Script integration** - Access env vars in JavaScript code
5. **Multi-environment support** - Same test file works across local/dev/staging/prod

## Best Practices

1. **Never commit .env files** - Always add `.env` to `.gitignore`
2. **Use .env.example** - Provide a template with dummy values
3. **Validate required vars** - Check critical env vars exist before running
4. **Environment-specific files** - Use `.env.local`, `.env.staging`, etc.
5. **CI/CD integration** - Use native secret management in your CI/CD platform