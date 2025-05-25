package runtime

import (
	"fmt"
)

// Context provides the runtime environment for script execution
type Context struct {
	// Input data (read-only)
	State map[string]string      // Current workflow state from previous steps
	Vars  map[string]interface{} // Configuration variables
	
	// Output data
	Saved map[string]string // Values to save back to workflow state
	
	// Runtime functions
	saveFunc   func(key, value string)
	assertFunc func(condition bool, message string) error
}

// NewContext creates a new runtime context
func NewContext(state map[string]string, vars map[string]interface{}) *Context {
	ctx := &Context{
		State: state,
		Vars:  vars,
		Saved: make(map[string]string),
	}
	
	// Set up built-in functions
	ctx.saveFunc = ctx.save
	ctx.assertFunc = ctx.assert
	
	return ctx
}

// Save stores a value in the workflow state for later steps
func (c *Context) Save(key, value string) {
	c.save(key, value)
}

// Assert validates a condition and throws an error if false
func (c *Context) Assert(condition bool, message string) error {
	return c.assert(condition, message)
}

// GetSaveFunc returns the save function for injection into script runtime
func (c *Context) GetSaveFunc() func(key, value string) {
	return c.saveFunc
}

// GetAssertFunc returns the assert function for injection into script runtime
func (c *Context) GetAssertFunc() func(condition bool, message string) error {
	return c.assertFunc
}

// save is the internal implementation of the save function
func (c *Context) save(key, value string) {
	if c.Saved == nil {
		c.Saved = make(map[string]string)
	}
	c.Saved[key] = value
}

// assert is the internal implementation of the assert function
func (c *Context) assert(condition bool, message string) error {
	if !condition {
		return fmt.Errorf("assertion failed: %s", message)
	}
	return nil
}