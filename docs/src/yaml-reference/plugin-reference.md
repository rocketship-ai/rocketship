
## Test Structure

| Field | Required | Description |
| ----- | -------- | ----------- |
| `name` | ✅ | Name of the test suite |
| `description` |  | Description of the test suite |
| `vars` |  | Configuration variables that can be referenced in test steps using {{ vars.key }} syntax |
| `tests` | ✅ | Array of test cases |


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
- `browser`
- `supabase`
- `mongodb`


---

## Plugin Configurations


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
| `agent` | ✅ | Type of coding agent to use | `claude-code` | - |
| `prompt` | ✅ | Prompt to send to the agent (supports template variables) | `string` | - |
| `mode` |  | Agent execution mode | `single`, `continue`, `resume` | - |
| `session_id` |  | Session ID for resume mode (supports template variables) | `string` | - |
| `max_turns` |  | Maximum number of conversation turns | `integer` | - |
| `timeout` |  | Agent execution timeout | `string` | - |
| `system_prompt` |  | System prompt for the agent | `string` | - |
| `output_format` |  | Output format from the agent | `text`, `json`, `streaming-json` | - |
| `continue_recent` |  | Continue the most recent conversation | `boolean` | - |
| `save_full_response` |  | Save the complete response to context | `boolean` | - |


### Plugin: `browser`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `task` | ✅ | Task description for the browser agent to perform (supports template variables) | `string` | - |
| `llm` | ✅ | No description | `object` | - |
| `llm.provider` | ✅ | LLM provider to use | `openai`, `anthropic` | - |
| `llm.model` | ✅ | LLM model to use (e.g., gpt-4, claude-3-sonnet) | `string` | - |
| `llm.config` |  | LLM configuration (API keys, etc.) | `object` | - |
| `executor_type` |  | Browser executor type | `python` | - |
| `timeout` |  | Browser automation timeout | `string` | - |
| `max_steps` |  | Maximum number of browser automation steps | `integer` | - |
| `browser_type` |  | Browser type to use | `chromium`, `chrome`, `edge` | - |
| `headless` |  | Run browser in headless mode | `boolean` | - |
| `use_vision` |  | Enable visual processing | `boolean` | - |
| `session_id` |  | Browser session ID for session persistence (supports template variables) | `string` | - |
| `save_screenshots` |  | Save screenshots during execution | `boolean` | - |
| `allowed_domains[]` |  | List of allowed domains for browser navigation | `array of string` | - |
| `viewport` |  | Browser viewport configuration | `object` | - |
| `viewport.width` |  | Viewport width in pixels | `integer` | - |
| `viewport.height` |  | Viewport height in pixels | `integer` | - |


### Plugin: `supabase`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `url` | ✅ | Supabase project URL | `string` | - |
| `key` | ✅ | Supabase API key (anon or service key) | `string` | - |
| `operation` | ✅ | Supabase operation to perform | `select`, `insert`, `update`, `delete`, `rpc`, `auth_create_user`, `auth_delete_user`, `auth_sign_up`, `auth_sign_in`, `storage_create_bucket`, `storage_upload`, `storage_download`, `storage_delete` | - |
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


### Plugin: `mongodb`

| Field | Required | Description | Type / Allowed Values | Notes |
| ----- | -------- | ----------- | --------------------- | ----- |
| `uri` | ✅ | MongoDB connection URI | `string` | - |
| `database` | ✅ | Database name | `string` | - |
| `collection` |  | Collection name for operations | `string` | - |
| `operation` | ✅ | MongoDB operation to perform | `insert`, `insert_many`, `find`, `find_one`, `update`, `update_many`, `delete`, `delete_many`, `count`, `aggregate`, `create_index`, `drop_index`, `list_indexes`, `create_collection`, `drop_collection`, `list_collections` | - |
| `insert` |  | Configuration for insert operations | `object` | - |
| `insert.document` |  | Single document to insert | `any` | - |
| `insert.documents[]` |  | Multiple documents to insert | `array of any` | - |
| `insert.ordered` |  | Whether to perform ordered insertion | `boolean` | - |
| `find` |  | Configuration for find operations | `object` | - |
| `find.filter` |  | Query filter | `object` | - |
| `find.projection` |  | Fields to include/exclude | `object` | - |
| `find.sort` |  | Sort specification | `object` | - |
| `find.limit` |  | Limit number of results | `integer` | - |
| `find.skip` |  | Number of documents to skip | `integer` | - |
| `update` |  | Configuration for update operations | `object` | - |
| `update.filter` | ✅ | Query filter | `object` | - |
| `update.update` | ✅ | Update document | `object` | - |
| `update.upsert` |  | Create if not exists | `boolean` | - |
| `update.multiple` |  | Update multiple documents | `boolean` | - |
| `delete` |  | Configuration for delete operations | `object` | - |
| `delete.filter` | ✅ | Query filter | `object` | - |
| `delete.multiple` |  | Delete multiple documents | `boolean` | - |
| `count` |  | Configuration for count operations | `object` | - |
| `count.filter` |  | Query filter | `object` | - |
| `aggregate` |  | Configuration for aggregate operations | `object` | - |
| `aggregate.pipeline[]` | ✅ | Aggregation pipeline | `array of object` | - |
| `index` |  | Configuration for index operations | `object` | - |
| `index.keys` |  | Index keys specification | `object` | - |
| `index.options` |  | Index options | `object` | - |
| `index.action` |  | Index action | `create`, `drop`, `list` | - |
| `index.name` |  | Index name for drop operation | `string` | - |
| `admin` |  | Configuration for admin operations | `object` | - |
| `admin.action` | ✅ | Admin action | `create_collection`, `drop_collection`, `list_collections` | - |
| `admin.collection` |  | Collection name | `string` | - |
| `timeout` |  | Operation timeout | `string` | - |


---

## Assertions

| Field | Required | Description | Allowed Values |
| ----- | -------- | ----------- | -------------- |
| `type` | ✅ | Type of assertion | `status_code`, `json_path`, `header`, `row_count`, `query_count`, `success_count`, `column_value`, `supabase_count`, `supabase_error`, `document_exists`, `count_equals`, `field_matches`, `result_contains`, `error_exists` |
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

