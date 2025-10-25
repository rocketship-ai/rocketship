
## Test Structure

| Field | Required | Description |
| ----- | -------- | ----------- |
| `name` | ✅ | Name of the test suite |
| `description` |  | Description of the test suite |
| `vars` |  | Configuration variables that can be referenced in test steps using {{ vars.key }} syntax |
| `openapi` |  | Suite-level OpenAPI contract validation defaults applied to HTTP steps |
| `init` |  | Suite-level initialization steps executed before any tests run |
| `tests` | ✅ | Array of test cases |
| `cleanup` |  | Suite-level cleanup hooks executed after all tests complete or when initialization fails |


---

## Test Step Structure

| Field | Required | Description |
| ----- | -------- | ----------- |
| `name` | ✅ | Name of the test step |
| `plugin` | ✅ | Plugin to use for this step |
| `config` | ✅ | Configuration for the plugin |
| `assertions` |  | Assertions to validate the response |
| `save` |  | Response values to save for use in later steps |
| `retry` |  | Retry policy for the step activity |


---

## Supported Plugins

- `http`
- `delay`
- `script`
- `sql`
- `log`
- `agent`
- `playwright`
- `browser_use`
- `supabase`


---

## Plugin Configurations


### Plugin: `http`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `method` | ✅ | HTTP method to use | `string` | - |
| `url` | ✅ | Request URL | `string` | - |
| `headers` |  | HTTP headers to include | `object` | - |
| `body` |  | Raw request body (string). If 'form' is also provided, 'form' takes precedence. | `string` | - |
| `form` |  | Form fields to be url-encoded as application/x-www-form-urlencoded | `object` | - |
| `openapi` |  | Override OpenAPI validation behavior for this HTTP step | `object` | - |
| `openapi.spec` |  | Path or URL to an OpenAPI v3 document | `string` | - |
| `openapi.operation_id` |  | Require the request to match a specific operationId | `string` | - |
| `openapi.version` |  | Optional spec version identifier used to invalidate cached contracts | `string` | - |
| `openapi.validate_request` |  | Enable request validation for this step | `boolean` | - |
| `openapi.validate_response` |  | Enable response validation for this step | `boolean` | - |


### Plugin: `script`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `language` | ✅ | Script language to use | `javascript`, `shell` | - |
| `script` |  (oneOf) | Inline script content | `string` | - |
| `file` |  (oneOf) | Path to external script file | `string` | - |
| `timeout` |  | Script execution timeout | `string` | - |


### Plugin: `sql`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `driver` | ✅ | Database driver to use | `postgres`, `mysql`, `sqlite`, `sqlserver` | - |
| `dsn` | ✅ | Database connection string (Data Source Name) | `string` | - |
| `commands[]` |  (oneOf) | Array of SQL commands to execute | `array of string` | - |
| `file` |  (oneOf) | Path to external SQL file | `string` | - |
| `timeout` |  | Query execution timeout | `string` | - |


##### `sql` Assertions

| Field | Required | Description | Allowed Values |
| ----- | -------- | ----------- | -------------- |
| `type` | ✅ | Type of SQL assertion | `row_count`, `query_count`, `success_count`, `column_value` |
| `expected` | ✅ | Expected value for the assertion | - |
| `query_index` |  (if `type` is `row_count`) (if `type` is `column_value`) | Index of query to check (for row_count and column_value assertions) | - |
| `row_index` |  (if `type` is `column_value`) | Index of row to check (for column_value assertion) | - |
| `column` |  (if `type` is `column_value`) | Column name to check (for column_value assertion) | - |


##### `sql` Save Fields

| Field | Required | Description | Notes |
| ----- | -------- | ----------- | ----- |
| `sql_result` | ✅ | Path to extract from SQL result (e.g., '.queries[0].rows[0].id') | - |
| `as` |  | Variable name to save the extracted value as | - |
| `required` |  | Whether the value is required (defaults to true) | - |


### Plugin: `log`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `message` | ✅ | Message to log (supports template variables) | `string` | - |


### Plugin: `agent`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `prompt` | ✅ | Task description for the agent (supports template variables) | `string` | - |
| `mode` |  | Execution mode (default: single) | `single`, `continue`, `resume` | - |
| `session_id` |  | Session ID for continue/resume modes (supports template variables) | `string` | - |
| `max_turns` |  | Maximum conversation turns (default: unlimited) | `integer` | - |
| `timeout` |  | Execution timeout (default: unlimited) | `string` | - |
| `system_prompt` |  | System prompt prepended to conversation (supports template variables) | `string` | - |
| `cwd` |  | Working directory for agent execution | `string` | - |
| `mcp_servers` |  | MCP server configurations | `object` | - |
| `allowed_tools` |  | Tool permissions (default: ['*'] wildcard) | `any` | - |


### Plugin: `supabase`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `url` |  | Supabase project URL (optional - auto-detected from SUPABASE_URL env var if not provided) | `string` | - |
| `key` |  | Supabase API key (optional - auto-detected from SUPABASE_SECRET_KEY, SUPABASE_SERVICE_KEY, SUPABASE_PUBLISHABLE_KEY, or SUPABASE_ANON_KEY env vars if not provided) | `string` | - |
| `operation` | ✅ | Supabase operation to perform | `select`, `insert`, `update`, `delete`, `rpc`, `auth_create_user`, `auth_delete_user`, `auth_sign_up`, `auth_sign_in`, `storage_create_bucket`, `storage_delete_bucket`, `storage_upload`, `storage_download`, `storage_delete` | - |
| `table` |  | Table name for database operations | `string` | - |
| `select` |  | Configuration for select operation | `object` | - |
| `select.columns[]` |  | Columns to select | `array of string` | - |
| `select.filters[]` |  | Filters to apply | `array of objects` | - |
| `select.filters[].column` | ✅ | No description | `string` | - |
| `select.filters[].operator` | ✅ | No description | `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `like`, `ilike`, `is`, `in`, `contains`, `contained_by`, `range_gt`, `range_gte`, `range_lt`, `range_lte` | - |
| `select.filters[].value` | ✅ | Filter value | `any` | - |
| `select.order[]` |  | Ordering configuration | `array of objects` | - |
| `select.order[].column` | ✅ | No description | `string` | - |
| `select.order[].ascending` |  | No description | `boolean` | - |
| `select.limit` |  | No description | `integer` | - |
| `select.offset` |  | No description | `integer` | - |
| `select.count` |  | Count type for query | `exact`, `planned`, `estimated` | - |
| `insert` |  | Configuration for insert operation | `object` | - |
| `insert.data` | ✅ | Data to insert (object or array of objects) | `any` | - |
| `insert.upsert` |  | Use upsert (insert or update) | `boolean` | - |
| `insert.on_conflict` |  | Column(s) for conflict resolution | `string` | - |
| `update` |  | Configuration for update operation | `object` | - |
| `update.data` | ✅ | Data to update | `object` | - |
| `update.filters[]` |  | Filters to apply for update | `array of objects` | - |
| `update.filters[].column` | ✅ | No description | `string` | - |
| `update.filters[].operator` | ✅ | No description | `string` | - |
| `update.filters[].value` | ✅ | Filter value | `any` | - |
| `delete` |  | Configuration for delete operation | `object` | - |
| `delete.filters[]` | ✅ | Filters to apply for delete | `array of objects` | - |
| `delete.filters[].column` | ✅ | No description | `string` | - |
| `delete.filters[].operator` | ✅ | No description | `string` | - |
| `delete.filters[].value` | ✅ | Filter value | `any` | - |
| `rpc` |  | Configuration for RPC function call | `object` | - |
| `rpc.function` | ✅ | Function name to call | `string` | - |
| `rpc.params` |  | Function parameters | `object` | - |
| `auth` |  | Configuration for auth operations | `object` | - |
| `auth.email` |  | No description | `string` | - |
| `auth.password` |  | No description | `string` | - |
| `auth.user_id` |  | No description | `string` | - |
| `auth.user_metadata` |  | No description | `object` | - |
| `auth.app_metadata` |  | No description | `object` | - |
| `storage` |  | Configuration for storage operations | `object` | - |
| `storage.bucket` |  | No description | `string` | - |
| `storage.path` |  | No description | `string` | - |
| `storage.file_content` |  | No description | `string` | - |
| `storage.file_path` |  | No description | `string` | - |
| `storage.public` |  | No description | `boolean` | - |
| `storage.cache_control` |  | No description | `string` | - |
| `storage.content_type` |  | No description | `string` | - |
| `timeout` |  | Operation timeout | `string` | - |


### Plugin: `delay`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `duration` | ✅ | Duration to delay (e.g., '5s', '1m', '2h') | `string` | - |


### Plugin: `playwright`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `session_id` | ✅ | Browser session identifier | `string` | - |
| `role` | ✅ | Playwright action: 'start' launches browser, 'script' runs Python code, 'stop' closes browser | `start`, `script`, `stop` | - |
| `headless` |  | Run browser in headless mode (for 'start' role) | `boolean` | - |
| `slow_mo_ms` |  | Slow down operations by N milliseconds (for 'start' role) | `integer` | - |
| `launch_args[]` |  | Additional browser launch arguments (for 'start' role) | `array of string` | - |
| `launch_timeout` |  | Browser launch timeout in milliseconds (for 'start' role) | `integer` | - |
| `window_width` |  | Browser window width in pixels (for 'start' role) | `integer` | - |
| `window_height` |  | Browser window height in pixels (for 'start' role) | `integer` | - |
| `script` |  | Python script to execute (for 'script' role) | `string` | - |
| `language` |  | Script language - only 'python' is supported (for 'script' role) | `python` | - |
| `env` |  | Environment variables for script execution (for 'script' role) | `object` | - |


### Plugin: `browser_use`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `session_id` | ✅ | Browser session identifier | `string` | - |
| `task` | ✅ | Natural language task for the AI agent to perform | `string` | - |
| `allowed_domains[]` |  | Restrict browser navigation to these domains | `array of string` | - |
| `max_steps` |  | Maximum number of agent steps | `integer` | - |
| `use_vision` |  | Enable vision capabilities for the AI agent | `boolean` | - |
| `temperature` |  | LLM temperature for agent decisions | `number` | - |
| `timeout` |  | Task execution timeout (e.g., '5m', '30s') | `string` | - |
| `llm` |  | LLM configuration for the agent | `object` | - |
| `llm.provider` |  | LLM provider (e.g., 'anthropic', 'openai') | `string` | - |
| `llm.model` |  | LLM model name | `string` | - |
| `llm.config` |  | LLM configuration (e.g., API keys as env vars) | `object` | - |


---

## Assertions

| Field | Required | Description | Allowed Values |
| ----- | -------- | ----------- | -------------- |
| `type` | ✅ | Type of assertion | `status_code`, `json_path`, `header`, `row_count`, `query_count`, `success_count`, `column_value`, `supabase_count`, `supabase_error` |
| `expected` | ✅ | Expected value for the assertion | - |
| `path` |  (if `type` is `json_path`) | JSON path for json_path assertion type | - |
| `name` |  (if `type` is `header`) | Header name for header assertion type | - |
| `query_index` |  (if `type` is `row_count`) (if `type` is `column_value`) | Index of query to check (for SQL assertions) | - |
| `row_index` |  (if `type` is `column_value`) | Index of row to check (for column_value assertion) | - |
| `column` |  (if `type` is `column_value`) | Column name to check (for column_value assertion) | - |


---

## Save Fields

| Field | Required | Description | Notes |
| ----- | -------- | ----------- | ----- |
| `json_path` |  (oneOf) | JSON path to extract from response | - |
| `header` |  (oneOf) | Header name to extract from response | - |
| `sql_result` |  (oneOf) | Path to extract from SQL result (e.g., '.queries[0].rows[0].id') | - |
| `as` | ✅ | Variable name to save the extracted value as | - |
| `required` |  | Whether the value is required (defaults to true) | - |

