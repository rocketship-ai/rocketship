# Contract Validation for HTTP Steps

Rocketship can validate every HTTP request and response against an OpenAPI v3 contract. The validator now lives at the suite level, so you configure it once and inherit the settings everywhere.

## Suite-Level Defaults

```yaml
openapi:
  spec: "./contracts/checkout.yaml"
  cache_ttl: "30m"
  validate_request: true
  validate_response: true
```

- `spec` accepts relative or absolute filesystem paths **and** HTTP(S) URLs.
- YAML and JSON OpenAPI documents are both supported; kin-openapi auto-detects the format.
- `cache_ttl` controls how long a contract stays cached (default 30 minutes). Raise or lower it depending on how often specs change.
- Bump the optional `version` string (`version: "2024-03-15"`) whenever you publish a new contract to force an immediate refresh.

## Step-Level Overrides

Sometimes you need to skip a check for a particular scenario (for example, sending intentionally malformed data). Add an `openapi` block on the step to toggle behaviour:

```yaml
- name: "Submit malformed payload"
  plugin: http
  config:
    method: POST
    url: https://api.example.com/orders
    body: '{"sku": "broken"}'
    openapi:
      validate_request: false
```

You can also point a specific step at a different `spec`, require an `operation_id`, or disable response validation.

## Example Combined with Request Chaining

The request-chaining suite uses a single contract:

```yaml
openapi:
  spec: "./contracts/tryme.yaml"
  cache_ttl: "30m"
```

Every HTTP step inherits that contract. One step turns off `validate_request` to test how the server handles malformed input:

```yaml
- name: "Create second car"
  plugin: http
  config:
    method: "POST"
    url: "https://tryme.rocketship.sh/cars"
    body: '{"make":"Honda","model":"Civic"}'
    openapi:
      validate_request: false
```

With the contract in place, Rocketship fails the run if the server returns extra fields, the wrong status code, or a payload that violates the schema. All failures include the underlying kin-openapi error details.

## Cache Behaviour

- Entries expire automatically after the configured `cache_ttl`.
- Local specs refresh as soon as their modification time changes.
- Remote specs refresh when the TTL expires or when you bump the `version` field.

## Next Steps

- Build the full [HTTP Request Chaining](request-chaining.md) flow.
- Combine delays and contract validation for resilient suites by pairing this guide with [Managing Delays](delays.md).
