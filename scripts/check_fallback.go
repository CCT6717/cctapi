package main

import (
	"fmt"
	"os"

	"github.com/songquanpeng/one-api/fallback"
)

func main() {
	// Load the fallback configuration
	fallbackPath := "data/fallback.json"
	if len(os.Args) > 1 {
		fallbackPath = os.Args[1]
	}

	fmt.Printf("Loading fallback configuration from: %s\n", fallbackPath)
	err := fallback.LoadConfig(fallbackPath)
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Configuration loaded successfully!")

	// Test IsEnabled
	if fallback.IsEnabled() {
		fmt.Println("✅ Fallback is enabled")
	} else {
		fmt.Println("❌ Fallback is not enabled")
		os.Exit(1)
	}

	// Test GetAllVirtualModelNames
	vmNames := fallback.GetAllVirtualModelNames()
	fmt.Printf("\n📋 Virtual models found: %v\n", vmNames)
	if len(vmNames) == 0 {
		fmt.Println("❌ No virtual models configured")
		os.Exit(1)
	}

	// Test IsVirtualModel
	testModels := []string{"high/auto", "low/auto", "all/auto", "gpt-3.5-turbo", "unknown-model"}
	for _, model := range testModels {
		if fallback.IsVirtualModel(model) {
			fmt.Printf("✅ '%s' is a virtual model\n", model)
		} else {
			fmt.Printf("  '%s' is not a virtual model\n", model)
		}
	}

	// Test each virtual model
	for _, vmName := range vmNames {
		fmt.Printf("\n=== Testing virtual model: %s ===\n", vmName)

		// Test GetVirtualModel
		vm, ok := fallback.GetVirtualModel(vmName)
		if ok {
			fmt.Printf("✅ Found: %s\n", vm.Description)
			fmt.Printf("   Fallback order: %v\n", vm.FallbackOrder)
		} else {
			fmt.Printf("❌ Virtual model '%s' not found\n", vmName)
			os.Exit(1)
		}

		// Test GetDeploymentsForVirtualModel
		deployments, err := fallback.GetDeploymentsForVirtualModel(vmName)
		if err != nil {
			fmt.Printf("❌ Failed to get deployments: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Found %d deployments:\n", len(deployments))
		for _, dep := range deployments {
			fmt.Printf("   - ID: %s, ChannelID: %d, Model: %s, Priority: %d, Weight: %d\n",
				dep.ID, dep.ChannelID, dep.RealModel, dep.Priority, dep.Weight)
		}
	}

	// Test GetDeployment on each known deployment
	knownDeployments := []string{"doubao-code", "doubao-18", "doubao-16", "openrouter-new-free", "openrouter-old", "openrouter-new"}
	for _, depName := range knownDeployments {
		dep, ok := fallback.GetDeployment(depName)
		if ok {
			fmt.Printf("✅ Found deployment: %s\n", dep.ID)
			fmt.Printf("   ChannelID: %d, RealModel: %s\n", dep.ChannelID, dep.RealModel)
		} else {
			fmt.Printf("❌ Deployment '%s' not found\n", depName)
			os.Exit(1)
		}
	}

	// Test ValidateConfig
	if err := fallback.ValidateConfig(); err != nil {
		fmt.Printf("❌ Config validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n🎉 All tests passed!")
	fmt.Printf("\n💡 Usage examples:\n")
	fmt.Printf("  if fallback.IsVirtualModel(\"high/auto\") {\n")
	fmt.Printf("      deployments, _ := fallback.GetDeploymentsForVirtualModel(\"high/auto\")\n")
	fmt.Printf("  }\n")
	fmt.Printf("  if fallback.IsVirtualModel(\"low/auto\") {\n")
	fmt.Printf("      deployments, _ := fallback.GetDeploymentsForVirtualModel(\"low/auto\")\n")
	fmt.Printf("  }\n")
	fmt.Printf("  if fallback.IsVirtualModel(\"all/auto\") {\n")
	fmt.Printf("      deployments, _ := fallback.GetDeploymentsForVirtualModel(\"all/auto\")\n")
	fmt.Printf("  }\n")
}
