# Browser Plugin Validation Analysis

**Date:** 2025-10-16
**Context:** Understanding why our current browser_use plugin validation is fragile and how to fix it

---

## TL;DR: We're Making the Same Mistake the Old Plugin Made

**The Problem:** Both the old `internal/plugins/browser` (deleted) and our new `internal/plugins/browser_use` plugin use **fragile string matching** to determine if a task succeeded or failed.

**The Solution:** Use **structured output with JSON schema** (like qa-use does) to force the agent to return machine-readable pass/fail status.

---

## The Old Browser Plugin (main branch)

### Architecture

**File:** `internal/plugins/browser/browser_automation.py`

**Approach:**
- Single Python script that launches browser-use Agent
- Runs task to completion in one execution
- Returns JSON response with success/failure
- **NO session management** - launches, runs, dies
- **NO playwright integration** - pure browser-use

### How It Checked Success/Failure

**Lines 287-299 of browser_automation.py:**

```python
# Build response - check if the result indicates success
success = True
result_str = str(result) if result else "Task completed successfully"

# Check if the result is a dict/object with a success field
if hasattr(result, '__dict__') and hasattr(result, 'success'):
    success = bool(result.success)
elif isinstance(result, dict) and 'success' in result:
    success = bool(result['success'])
# If result is a string containing "success: false" or similar
elif isinstance(result, str):
    result_lower = result.lower()
    if "success: false" in result_lower or "success\":false" in result_lower:
        success = False
    elif "failed" in result_lower or "error" in result_lower or "could not" in result_lower:
        success = False  # ❌ FRAGILE STRING MATCHING!
```

**Problems with this approach:**
1. ❌ String matching on "failed", "error", "could not"
2. ❌ False positives (agent describing error handling as expected behavior)
3. ❌ False negatives (agent failing but not using these words)
4. ❌ LLM version dependent
5. ❌ Language dependent

### What It Did Right

1. ✅ Simple architecture - no session files
2. ✅ Environment variable config (clean)
3. ✅ Embedded Python script (portable)
4. ✅ Returned structured BrowserResponse
5. ✅ Had save/assertion support

---

## Our Current browser_use Plugin

### Architecture

**File:** `internal/plugins/browser_use/browser_use_runner.py`

**Approach:**
- Shares browser session with playwright plugin
- Connects to existing CDP endpoint
- Returns JSON response with success/failure

### How It Checks Success/Failure

**Lines 163-184 of browser_use_runner.py:**

```python
# Check if task actually completed
task_completed = result.is_done() if hasattr(result, 'is_done') else False

# Additionally check if the final result indicates failure
error_message = None
if hasattr(result, 'final_result'):
    final_text = str(result.final_result()).lower()
    # Check for common failure indicators in the agent's response
    failure_indicators = [
        'unable to',
        'could not',
        'couldn\'t',
        'cannot',
        'can\'t',
        'did not find',
        'didn\'t find',
        'not found',
        'failed to',
        'failure',
    ]
    if any(indicator in final_text for indicator in failure_indicators):
        task_completed = False  # ❌ FRAGILE STRING MATCHING!
        error_message = f"Agent reported task failure: {result.final_result()}"
```

**This is THE SAME PROBLEM as the old plugin!**

### Problems with This Approach

#### 1. False Positives

Agent describing **expected behavior** gets marked as failure:

```python
# Agent testing error handling:
"The form correctly shows 'unable to submit' when validation fails"
# ❌ MARKED AS FAILURE (contains "unable to")

# Agent documenting API behavior:
"Verified that API returns 404 when resource is not found"
# ❌ MARKED AS FAILURE (contains "not found")

# Agent testing invalid input:
"Confirmed the system cannot process invalid dates as expected"
# ❌ MARKED AS FAILURE (contains "cannot")
```

#### 2. False Negatives

Agent **actually failing** but gets marked as success:

```python
# Agent giving up:
"I didn't complete the requested action"
# ✅ MARKED AS SUCCESS (doesn't match any indicator)

# Agent reporting failure:
"The task was unsuccessful"
# ✅ MARKED AS SUCCESS (doesn't match)

# Agent hitting limits:
"Unfortunately I ran into issues and gave up"
# ✅ MARKED AS SUCCESS (doesn't match "unable to")
```

#### 3. LLM Version Dependency

```python
# GPT-4:
"I was unable to find the element"
# ❌ CAUGHT by "unable to"

# GPT-5 (hypothetical future):
"The element wasn't located"
# ✅ MISSED (different phrasing)

# Claude:
"I couldn't verify this requirement"
# ❌ CAUGHT by "couldn't"
# BUT: "I was unsuccessful in locating the heading"
# ✅ MISSED (different phrasing)
```

#### 4. Language/Localization Issues

If a customer wants to use browser_use in another language:

```python
# French LLM:
"Je n'ai pas pu trouver l'élément"
# ✅ MISSED (not English)

# Spanish LLM:
"No pude completar la tarea"
# ✅ MISSED (not English)
```

---

## What browser_use Actually Provides

### AgentHistoryList Methods (from GitHub)

```python
# Agent execution result
result = await agent.run(max_steps=30)

# ✅ RELIABLE: Structured state checking
is_done = result.is_done()  # Boolean: Did agent finish?
is_successful = result.is_successful()  # Boolean | None: Was it successful?
has_errors = result.has_errors()  # Boolean: Any errors?
errors = result.errors()  # List of error objects

# ❌ UNRELIABLE: Natural language output
final_result = result.final_result()  # String: Agent's natural language description
```

### The Problem with Our Current Code

We're using **both** `is_done()` (reliable) AND string matching on `final_result()` (unreliable):

```python
# Line 163: Good approach
task_completed = result.is_done() if hasattr(result, 'is_done') else False

# Lines 167-184: Bad approach - overrides the good approach!
if hasattr(result, 'final_result'):
    final_text = str(result.final_result()).lower()
    if any(indicator in final_text for indicator in failure_indicators):
        task_completed = False  # ❌ Overriding is_done() with string matching!
```

**Why is this bad?**

`is_done()` returns `True` when the agent believes it finished the task. Then we **override** that judgment by parsing the natural language description for failure words.

**Scenario:**
```python
# Agent successfully completes a test:
# Task: "Go to login page, test that it shows error when credentials are invalid"

# Agent executes perfectly and sets is_done=True
result.is_done() # ✅ True

# Agent describes what it did:
result.final_result() # "I verified the login page shows 'Unable to login' error with invalid credentials"

# Our code:
task_completed = True  # from is_done()
# Then overrides with string matching:
"unable to" in final_result  # True
task_completed = False  # ❌ WRONG! Task succeeded but we marked it as failed
```

---

## The qa-use Solution (CORRECT Approach)

### How qa-use Validates Tests

**File:** `qa-use/src/lib/testing/engine.ts`

**Key Insight:** Use **JSON Schema** to force structured output from the agent.

### 1. Define Strict Response Schema

```typescript
export const zResponse = z.object({
  status: z.union([z.literal('pass'), z.literal('failing')]),
  steps: z.array(
    z.object({
      id: z.string(),
      description: z.string(),
    })
  ).nullable(),
  error: z.string().nullable(),
})

export const RESPONSE_JSON_SCHEMA = z.toJSONSchema(zResponse)
```

### 2. System Prompt Forces Structured Output

```
You are a testing agent that validates whether an application works as expected.

RESPONSE FORMAT (STRICTLY FOLLOW):
{
  "status": "pass" | "failing",
  "steps": [...completed steps...] | null,
  "error": "error description" | null
}

DO NOT INCLUDE ANY OTHER TEXT IN YOUR RESPONSE!
STRICTLY FOLLOW THE RESPONSE FORMAT DEFINED ABOVE!
```

### 3. Send JSON Schema to API

```typescript
const response = await fetch('https://api.browser-use.com/api/v1/run-task', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    task: getTaskPrompt(testDefinition),
    llm_model: 'o3',
    structured_output_json: JSON.stringify(RESPONSE_JSON_SCHEMA)  // ✅ Forces schema
  })
})
```

### 4. Parse and Validate Response

```typescript
export function getTaskResponse(output: string | null): TaskResponse {
  if (!output) {
    return { status: 'failing', steps: [], error: 'No output was provided!' }
  }

  try {
    const parsed = JSON.parse(output)
    const response = zResponse.safeParse(parsed)  // ✅ Validates against schema

    if (!response.success) {
      return {
        status: 'failing',
        steps: [],
        error: `Failed to parse task response: ${response.error.message}`
      }
    }

    return response.data  // ✅ Guaranteed to match schema
  } catch {
    return { status: 'failing', steps: [], error: 'Failed to parse task response' }
  }
}
```

### Why This Works

1. ✅ **No string matching** - agent returns machine-readable status
2. ✅ **LLM version independent** - schema is explicit contract
3. ✅ **Language independent** - JSON is universal
4. ✅ **No false positives** - agent can't accidentally trigger failure
5. ✅ **No false negatives** - agent must explicitly set status
6. ✅ **Validation** - Zod ensures response matches schema
7. ✅ **Fallback** - Invalid responses automatically marked as failing

---

## What We Should Do

### Option 1: Use browser-use's Built-in Structured Output (RECOMMENDED)

browser-use supports Pydantic models for structured output (from `examples/features/custom_output.py`):

```python
from pydantic import BaseModel
from browser_use import Agent

# Define output schema
class TaskResult(BaseModel):
    status: str  # "pass" or "failing"
    extracted_content: dict | None
    error: str | None

# Create agent with output schema
agent = Agent(
    task=enhanced_task,
    llm=llm,
    browser_session=session,
    output_model_schema=TaskResult  # ✅ Forces structured output
)

result = await agent.run(max_steps=max_steps)

# Extract validated result
task_result = TaskResult.model_validate_json(result.final_result())

# ✅ NO STRING MATCHING - use structured data
payload = {
    "ok": task_result.status == "pass",
    "result": task_result.extracted_content,
    "error": task_result.error,
}
```

### Option 2: Enhanced System Prompt (Simpler, Less Reliable)

If we can't use structured output, at least improve the prompt:

```python
# Current code (line 137-142)
agent_kwargs["extend_system_message"] = (
    "\n\nIMPORTANT: You MUST complete the full task as specified. "
    "If you cannot complete ANY part of the task due to missing elements, "
    "errors, or inability to find required content, you should report this as a failure. "
    "Do NOT report success if you only partially completed the task or could not find required elements."
)

# Improved version with structured output instruction
agent_kwargs["extend_system_message"] = (
    "\n\nIMPORTANT: At the end of your task execution, you MUST return a JSON object with this exact structure:\n"
    '{\n'
    '  "status": "pass" OR "failing",\n'
    '  "content": "what you extracted or accomplished",\n'
    '  "error": "error description if failing, otherwise null"\n'
    '}\n\n'
    "Set status to 'pass' ONLY if you successfully completed the ENTIRE task as specified.\n"
    "Set status to 'failing' if you:\n"
    "- Could not find required elements\n"
    "- Encountered errors\n"
    "- Only partially completed the task\n"
    "- Were unable to verify success criteria\n\n"
    "DO NOT include any other text outside the JSON object."
)
```

Then parse the JSON:

```python
# Instead of string matching (lines 167-184)
try:
    # Try to parse final_result as JSON
    result_json = json.loads(result.final_result())
    if isinstance(result_json, dict) and 'status' in result_json:
        task_completed = result_json['status'] == 'pass'
        if not task_completed:
            error_message = result_json.get('error', 'Task marked as failing')
    else:
        # Fallback to is_done if no structured output
        task_completed = result.is_done()
except (json.JSONDecodeError, AttributeError):
    # Fallback to is_done if JSON parsing fails
    task_completed = result.is_done()
```

### Option 3: Rely on browser_use's Built-in State (Simplest)

Just trust `is_done()` and `errors()` without any string matching:

```python
# Check if task actually completed
task_completed = result.is_done() if hasattr(result, 'is_done') else False

# Check for errors in the result
error_message = None
if hasattr(result, 'errors'):
    errors = result.errors()
    if errors and any(e is not None for e in errors):
        task_completed = False
        error_message = f"Agent encountered errors: {errors}"

# ❌ REMOVE LINES 167-184 (the string matching code)
```

---

## Comparison Matrix

| Approach | Reliability | False Positives | False Negatives | LLM Independent | Implementation Complexity |
|----------|------------|----------------|----------------|----------------|--------------------------|
| **Current (string matching)** | ❌ Low | ❌ High | ❌ High | ❌ No | Low |
| **Old browser plugin** | ❌ Low | ❌ High | ❌ High | ❌ No | Low |
| **Option 1: Pydantic schema** | ✅ High | ✅ None | ✅ None | ✅ Yes | Medium |
| **Option 2: JSON prompt** | 🟡 Medium | 🟡 Low | 🟡 Low | ✅ Yes | Medium |
| **Option 3: Trust is_done()** | 🟡 Medium | ✅ None | 🟡 Medium | ✅ Yes | Very Low |

---

## Recommendations

### Immediate Fix (1 hour)

**Remove the string matching code** (lines 167-184 in `browser_use_runner.py`):

```python
# BEFORE (lines 163-193):
task_completed = result.is_done() if hasattr(result, 'is_done') else False

# Additionally check if the final result indicates failure
error_message = None
if hasattr(result, 'final_result'):
    final_text = str(result.final_result()).lower()
    failure_indicators = [...]  # ❌ DELETE THIS
    if any(indicator in final_text for indicator in failure_indicators):
        task_completed = False
        error_message = f"Agent reported task failure: {result.final_result()}"

# Check for errors in the result
if hasattr(result, 'errors'):
    errors = result.errors()
    if errors and any(e is not None for e in errors):
        task_completed = False
        if not error_message:
            error_message = f"Agent encountered errors: {errors}"

# AFTER (simplified):
task_completed = result.is_done() if hasattr(result, 'is_done') else False

error_message = None
if hasattr(result, 'errors'):
    errors = result.errors()
    if errors and any(e is not None for e in errors):
        task_completed = False
        error_message = f"Agent encountered errors: {errors}"
```

**Why this is safe:**
- `is_done()` is browser_use's official API for completion checking
- `errors()` contains actual error objects, not string matching
- Removes all false positive/negative issues

### Short-term Fix (Half day)

**Implement Option 1: Pydantic structured output**

1. Define Pydantic model in Python:
```python
from pydantic import BaseModel

class TaskResult(BaseModel):
    status: str  # "pass" or "failing"
    extracted_content: dict | None = None
    error: str | None = None
```

2. Pass to agent:
```python
agent = Agent(
    task=args.task,
    llm=llm,
    browser_session=session,
    output_model_schema=TaskResult,  # ✅ Add this
    extend_system_message=...
)
```

3. Parse validated output:
```python
task_result = TaskResult.model_validate_json(result.final_result())
payload = {
    "ok": task_result.status == "pass",
    "result": _serialize(result),
    "finalUrl": final_url,
}
if task_result.status != "pass":
    payload["error"] = task_result.error or "Task did not complete successfully"
```

### Long-term Solution (1-2 days)

**Integrate qa-use patterns into Rocketship:**

1. Create reusable test validation schema (similar to qa-use's `engine.ts`)
2. Support both simple tasks (current) and structured test definitions (qa-use style)
3. Add proper step tracking and partial success detection
4. Implement retry logic for flaky browser operations

---

## Conclusion

**You are 100% correct** - the current string matching approach in `browser_use_runner.py` is:

1. ❌ Not scalable
2. ❌ Not production-ready
3. ❌ Fragile and error-prone
4. ❌ The same mistake the old browser plugin made

**The fix is simple:** Remove string matching, trust browser_use's built-in APIs (`is_done()`, `errors()`), and ideally use structured output via Pydantic models.

**qa-use shows us the gold standard:** Structured output with JSON schema validation eliminates ALL the fragility issues while being LLM-agnostic and language-independent.
