# Structured Output Testing Plan

**Date:** 2025-10-16
**Status:** Testing Phase
**Goal:** Verify browser-use `output_model_schema` reliability before implementing qa-use pattern

---

## The Question We're Answering

**Does browser-use's `output_model_schema` parameter actually enforce structured output, or is it just prompting?**

This determines whether we can safely use the qa-use pattern in production.

---

## Test Methodology

### Test Script: `test-structured-output-reliability.py`

**What it does:**

1. Runs 7 different scenarios with varying complexity
2. Each scenario uses `output_model_schema=TestResult` (Pydantic model)
3. Attempts to parse the response with Pydantic validation
4. Tracks success/failure rate
5. Provides interpretation of results

### Test Scenarios

#### 1. Simple Task (Baseline)
```python
task = "Say hello and return status as 'pass'"
# Expected: Should easily return valid JSON
```

#### 2. Explicit Fail Request
```python
task = "Return status as 'fail' with error message 'test error'"
# Expected: Should return {"status": "fail", "error": "test error"}
```

#### 3. Tricky Wording
```python
task = "Status should be 'pass'. Also, include the word 'fail' in your message."
# Tests: Can it distinguish between status='fail' vs mentioning "fail" in text?
# Prompting-only: Might get confused and set status='fail'
# Native enforcement: Will correctly set status='pass'
```

#### 4. Request for Extra Fields
```python
task = "Return status 'pass' and add an extra field 'bonus' with value 'extra data'"
# Tests: Does schema reject additionalProperties?
# Native enforcement: Extra field rejected or ignored
# Prompting-only: Might include extra field
```

#### 5. Request for Invalid Status
```python
task = "Set status to 'success' instead of 'pass'"
# Tests: Is enum enforced? (status must be "pass" or "fail")
# Native enforcement: Will fail validation or force valid value
# Prompting-only: Might return "success" and break validation
```

#### 6. Request for Non-JSON Response
```python
task = "Instead of JSON, just say 'The test passed successfully' in plain text"
# Tests: Is JSON format enforced?
# Native enforcement: Will return JSON despite instruction
# Prompting-only: Might return plain text
```

#### 7. Complex Natural Language Task
```python
task = "Check if 2+2 equals 4. If yes, return pass status. Include calculation in message."
# Tests: Can it handle realistic QA scenario?
# Expected: Should work regardless of enforcement method
```

---

## Interpreting Results

### Scenario A: 100% Success Rate

**Conclusion:** ✅ Native structured output enforcement

**What this means:**
- browser-use uses OpenAI/Anthropic native structured output APIs
- Schema is enforced at the LLM token generation level
- Invalid tokens are given 0% probability
- **100% reliable for production use**

**Evidence:**
- Even tricky scenarios (3, 5, 6) pass validation
- Extra fields (scenario 4) are rejected or ignored
- Invalid enum values (scenario 5) are corrected or fail gracefully

**Next step:** ✅ Proceed with qa-use pattern implementation

---

### Scenario B: 90-99% Success Rate

**Conclusion:** 🟡 Prompting + validation + retry logic

**What this means:**
- browser-use likely prompts for JSON format
- Has retry logic if validation fails
- Generally reliable but not 100% guaranteed

**Evidence:**
- Most scenarios pass
- Occasional failures on tricky scenarios (3, 5, 6)
- Might see different results on repeated runs

**Next step:**
- Run test multiple times to check consistency
- Read browser-use source code to confirm retry mechanism
- Decide if 90-99% is acceptable for production

---

### Scenario C: 70-89% Success Rate

**Conclusion:** ⚠️ Prompting only (no retry)

**What this means:**
- browser-use adds prompt instructions for JSON format
- No validation or retry logic
- **Not reliable enough for production QA testing**

**Evidence:**
- Failures on tricky scenarios (3, 5, 6)
- Inconsistent results on repeated runs
- Simple scenarios (1, 2, 7) might still work

**Next step:**
- ❌ Don't use `output_model_schema` as-is
- Read browser-use source code
- Implement our own enforcement (LangChain `.with_structured_output()`)

---

### Scenario D: <70% Success Rate

**Conclusion:** ❌ Broken or not working as expected

**What this means:**
- `output_model_schema` parameter is not working
- Might be a version mismatch or bug
- **Cannot use for production**

**Evidence:**
- Even simple scenarios fail
- Responses are not even JSON

**Next step:**
- Check browser-use version (need 0.8.0+)
- Read source code to understand what's broken
- Consider alternative approaches (old browser plugin style)

---

## How to Run the Test

### Prerequisites

```bash
# Install dependencies if not already installed
pip install browser-use langchain-openai

# Set API key
export OPENAI_API_KEY=your-actual-api-key
```

### Run Test

```bash
cd /Users/magius/Downloads/personal_projects/rocketship-ai/rocketship

# Run the test
python3 test-structured-output-reliability.py
```

### Expected Runtime

- ~1-2 minutes total
- 7 scenarios × ~10-15 seconds each
- Uses minimal LLM calls (max_steps=1 per test)

---

## Decision Tree Based on Results

```
Run test
    │
    ├─ 100% success → ✅ Use output_model_schema (qa-use pattern)
    │                  └─ Implement structured output
    │                     └─ Remove string matching code
    │                        └─ Ship to production
    │
    ├─ 90-99% success → Run test 3 more times
    │                    │
    │                    ├─ Consistent? → 🟡 Use with caution
    │                    │                └─ Add fallback validation
    │                    │                   └─ Log failures for monitoring
    │                    │
    │                    └─ Inconsistent? → Read source code
    │                                        └─ Understand retry logic
    │                                           └─ Decide if acceptable
    │
    ├─ 70-89% success → ❌ Don't use output_model_schema
    │                   └─ Read browser-use source
    │                      └─ Implement LangChain .with_structured_output()
    │                         └─ Test again
    │
    └─ <70% success → 🔍 Investigate
                      └─ Check versions
                         └─ Read source code
                            └─ File bug report?
```

---

## What We'll Learn

### If Native Enforcement Works (100% success):

✅ **We can trust qa-use's approach**
- They're using the same browser-use library
- They rely on `structured_output_json` in their Cloud API
- Cloud API likely wraps the same native LLM features
- Safe to implement in Rocketship

✅ **Implementation is straightforward**
```python
class BrowserTestResult(BaseModel):
    status: Literal["pass", "fail"]
    steps_completed: list[str] | None = None
    error: str | None = None

agent = Agent(
    task=task,
    llm=llm,
    output_model_schema=BrowserTestResult  # ✅ Enforced
)

result = await agent.run()
test_result = BrowserTestResult.model_validate_json(result.final_result())
# ✅ Always valid (or raises ValidationError for graceful handling)
```

✅ **No string matching needed**
- Status is guaranteed to be "pass" or "fail"
- Error field is guaranteed to exist (null or string)
- Can safely check `test_result.status == "pass"`

### If Native Enforcement Doesn't Work (<100% success):

❌ **We need deeper investigation**
- Read browser-use source code
- Understand their implementation
- Might need to implement our own enforcement

❌ **qa-use pattern needs adaptation**
- Can't blindly copy their approach
- Need retry logic or validation
- Might need to use LangChain directly

❌ **String matching alternative**
- Keep `is_done()` + `errors()` approach
- Skip structured output for now
- Document limitation for future improvement

---

## Code Locations to Read (If Needed)

If test results are inconclusive, read these files in browser-use source:

```
browser-use/browser_use/agent/
├── service.py           # Main Agent class
│   └─ Look for: output_model_schema handling
│   └─ Look for: LangChain integration
│   └─ Look for: JSON schema generation
│
├── views.py             # Result classes (AgentHistoryList, etc.)
│   └─ Look for: final_result() implementation
│
└── message_manager.py   # Prompt construction
    └─ Look for: System message for structured output
```

Check if they use:
```python
# Native enforcement (good)
self.llm.with_structured_output(output_model_schema)

# Or just prompting (bad)
system_message += f"Return JSON matching schema: {schema}"
```

---

## Success Criteria

**Test passes if:**
- All 7 scenarios return valid JSON
- Pydantic validation succeeds 100% of the time
- Tricky scenarios (3, 5, 6) don't break enforcement

**Test fails if:**
- Any scenario returns non-JSON
- Invalid enum values accepted (scenario 5)
- Plain text response returned (scenario 6)

**Inconclusive if:**
- Success rate between 70-99%
- Results vary on repeated runs
- Only simple scenarios pass

---

## Timeline

1. **Now:** Run test (2 minutes)
2. **If 100%:** Implement qa-use pattern (half day)
3. **If <100%:** Read source code (1-2 hours)
4. **Then:** Decide on implementation approach
5. **Finally:** Implement + test + ship

---

## Fallback Plan

If `output_model_schema` is unreliable, we have options:

### Option 1: Manual LangChain Enforcement
```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(model="gpt-4o")
structured_llm = llm.with_structured_output(BrowserTestResult)

# Use structured_llm directly instead of through browser-use
```

### Option 2: Simplified Trust (Current Best Alternative)
```python
# Just use is_done() and errors() - no string matching
task_completed = result.is_done()
errors = result.errors() if hasattr(result, 'errors') else []

if errors:
    task_completed = False
```

### Option 3: Retry Logic
```python
for attempt in range(3):
    result = await agent.run()
    try:
        test_result = BrowserTestResult.model_validate_json(result.final_result())
        break
    except ValidationError:
        if attempt == 2:
            # Give up after 3 attempts
            test_result = BrowserTestResult(status="fail", error="Invalid response format")
```

---

## Questions This Test Answers

1. ✅ Does `output_model_schema` enforce structure? → **Yes/No + reliability %**
2. ✅ Can we trust it for production? → **Based on success rate**
3. ✅ Is qa-use's approach valid for us? → **If they use same library features**
4. ✅ Do we need to read source code? → **If success rate < 100%**
5. ✅ What's our implementation path? → **Clear decision tree**

---

## Expected Outcome

**My prediction:** 100% success rate

**Why:**
- browser-use is production software (7k+ stars on GitHub)
- qa-use relies on structured output
- Modern LLMs (GPT-4o, Claude) have native support
- Would be strange to have `output_model_schema` parameter that doesn't work

**But we verify rather than assume** - hence this test.

If I'm wrong and it's <100%, we'll know exactly what to do next.

---

## After Testing

Once we have results:

1. **Document findings** in this file
2. **Update implementation plan** based on results
3. **Share test output** with team
4. **Proceed** with appropriate implementation path

Let's run the test! 🧪
