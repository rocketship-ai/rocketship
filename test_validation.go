package main

import (
	"fmt"
	"os"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

func main() {
	// Test with the existing complex example
	yamlData, err := os.ReadFile("examples/complex-http/rocketship.yaml")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	config, err := dsl.ParseYAML(yamlData)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Validation succeeded for: %s\n", config.Name)
	fmt.Printf("   Version: %s\n", config.Version)
	fmt.Printf("   Tests: %d\n", len(config.Tests))

	// Test with the simple delay example
	yamlData, err = os.ReadFile("examples/simple-delay/rocketship.yaml")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	config, err = dsl.ParseYAML(yamlData)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Validation succeeded for: %s\n", config.Name)
	fmt.Printf("   Version: %s\n", config.Version)
	fmt.Printf("   Tests: %d\n", len(config.Tests))

	// Test with an invalid example
	invalidYAML := []byte(`
name: "Invalid Test"
tests: []
`)

	_, err = dsl.ParseYAML(invalidYAML)
	if err != nil {
		fmt.Printf("✅ Correctly rejected invalid YAML: %v\n", err)
	} else {
		fmt.Printf("❌ Should have rejected invalid YAML\n")
		os.Exit(1)
	}

	fmt.Println("🎉 All validation tests passed!")
}