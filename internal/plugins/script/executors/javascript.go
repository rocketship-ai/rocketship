package executors

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/rocketship-ai/rocketship/internal/plugins/script/runtime"
)

// JavaScriptExecutor executes JavaScript code using the goja engine
type JavaScriptExecutor struct{}

// NewJavaScriptExecutor creates a new JavaScript executor
func NewJavaScriptExecutor() *JavaScriptExecutor {
	return &JavaScriptExecutor{}
}

// Language returns the language identifier
func (e *JavaScriptExecutor) Language() string {
	return "javascript"
}

// ValidateScript performs static validation of JavaScript code
func (e *JavaScriptExecutor) ValidateScript(script string) error {
	// Basic validation - try to parse the script
	_, err := goja.Compile("validation", script, false)
	if err != nil {
		return fmt.Errorf("javascript syntax error: %w", err)
	}
	return nil
}

// Execute runs the JavaScript code in the provided runtime context
func (e *JavaScriptExecutor) Execute(ctx context.Context, script string, rtCtx *runtime.Context) error {
	// Create timeout context
	timeout := 30 * time.Second // Default timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create JavaScript VM
	vm := goja.New()

	// Set up built-in functions
	if err := e.setupBuiltins(vm, rtCtx); err != nil {
		return fmt.Errorf("failed to setup built-ins: %w", err)
	}

	// Set up runtime data
	if err := e.setupRuntimeData(vm, rtCtx); err != nil {
		return fmt.Errorf("failed to setup runtime data: %w", err)
	}

	// Execute script with timeout
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("javascript panic: %v", r)
			}
		}()
		
		_, err := vm.RunString(script)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("javascript execution error: %w", err)
		}
		return nil
	case <-execCtx.Done():
		return fmt.Errorf("javascript execution timeout")
	}
}

// setupBuiltins injects built-in functions into the JavaScript runtime
func (e *JavaScriptExecutor) setupBuiltins(vm *goja.Runtime, rtCtx *runtime.Context) error {
	// Inject save function
	_ = vm.Set("save", func(key, value string) {
		rtCtx.Save(key, value)
	})

	// Inject assert function
	_ = vm.Set("assert", func(condition bool, message string) {
		err := rtCtx.Assert(condition, message)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
	})

	// Inject console object for logging
	console := vm.NewObject()
	_ = console.Set("log", func(args ...interface{}) {
		// For now, just ignore console.log calls
		// In the future, we could implement proper logging
	})
	_ = vm.Set("console", console)

	return nil
}

// setupRuntimeData injects runtime data (state, vars) into the JavaScript runtime
func (e *JavaScriptExecutor) setupRuntimeData(vm *goja.Runtime, rtCtx *runtime.Context) error {
	// Inject workflow state
	stateObj := vm.NewObject()
	for key, value := range rtCtx.State {
		_ = stateObj.Set(key, value)
	}
	_ = vm.Set("state", stateObj)

	// Inject configuration variables
	varsObj := e.convertToGojaValue(vm, rtCtx.Vars)
	_ = vm.Set("vars", varsObj)

	return nil
}

// convertToGojaValue recursively converts Go values to goja values
func (e *JavaScriptExecutor) convertToGojaValue(vm *goja.Runtime, value interface{}) goja.Value {
	switch v := value.(type) {
	case map[string]interface{}:
		obj := vm.NewObject()
		for key, val := range v {
			_ = obj.Set(key, e.convertToGojaValue(vm, val))
		}
		return obj
	case []interface{}:
		arr := vm.NewArray()
		for i, val := range v {
			_ = arr.Set(fmt.Sprintf("%d", i), e.convertToGojaValue(vm, val))
		}
		return arr
	default:
		return vm.ToValue(v)
	}
}