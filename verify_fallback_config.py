import json
import os
import sys

def test_fallback_config():
    """Test if the fallback configuration is properly loaded"""

    # Check if files exist
    config_path = "data/fallback.json"

    print("Testing Fallback Configuration...")
    print(f"Configuration path: {config_path}")
    print("-" * 50)

    if not os.path.exists(config_path):
        print(f"[X] Configuration file not found at: {config_path}")
        return False

    print("[OK] Configuration file found")

    # Check if config file is valid JSON
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            config = json.load(f)
        print("[OK] Configuration file is valid JSON")
    except json.JSONDecodeError as e:
        print(f"[X] Invalid JSON: {e}")
        return False

    # Check required fields
    required_fields = ['enabled', 'virtual_models', 'deployments']
    for field in required_fields:
        if field not in config:
            print(f"[X] Missing required field: {field}")
            return False
        print(f"[OK] Field '{field}' present")

    # Check enabled flag
    if not config['enabled']:
        print("[WARN] Fallback is disabled in configuration")
        return False
    print("[OK] Fallback is enabled")

    # Check virtual models
    if not config['virtual_models']:
        print("[X] No virtual models configured")
        return False
    print(f"[OK] Found {len(config['virtual_models'])} virtual model(s)")

    # Check each virtual model
    test_models = ["high/auto", "low/auto", "all/auto"]
    for test_model in test_models:
        if test_model not in config['virtual_models']:
            print(f"[X] Test model '{test_model}' not found")
            return False
        print(f"[OK] Test model '{test_model}' is configured")

        vm_config = config['virtual_models'][test_model]
        if not vm_config.get('enabled', False):
            print(f"[WARN] Model '{test_model}' is disabled")
            return False
        print(f"[OK] Model '{test_model}' is enabled")

        # Check fallback order
        fallback_order = vm_config.get('fallback_order', [])
        if not fallback_order:
            print(f"[WARN] Model '{test_model}' has empty fallback_order")
            return False
        print(f"[OK] Fallback order for '{test_model}': {fallback_order}")

        # Validate each deployment in order
        print(f"  Validating deployments:")
        for i, dep_id in enumerate(fallback_order, 1):
            if dep_id in config['deployments']:
                dep = config['deployments'][dep_id]
                print(f"    {i}. {dep_id}: Channel {dep.get('channel_id', 'N/A')}, Model {dep.get('real_model', 'N/A')}")
            else:
                print(f"    [X] {dep_id}: Deployment not found")
                return False

    print("\n" + "=" * 50)
    print("[SUCCESS] All tests passed!")
    print("\nConfiguration Summary:")
    print(f"  - Enabled: {config['enabled']}")
    print(f"  - Virtual models: {list(config['virtual_models'].keys())}")
    print(f"  - Deployments: {list(config['deployments'].keys())}")
    return True

if __name__ == "__main__":
    success = test_fallback_config()
    sys.exit(0 if success else 1)
