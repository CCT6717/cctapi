package freeproviderquirks

import "testing"

func TestForProviderReturnsKnownQuirks(t *testing.T) {
	quirks, ok := ForProvider("nvidia")
	if !ok {
		t.Fatal("expected nvidia quirks")
	}
	if quirks.ForceParallelToolCalls == nil || *quirks.ForceParallelToolCalls {
		t.Fatalf("expected nvidia to force parallel_tool_calls=false, got %+v", quirks)
	}
}

func TestFromAutoChannelNameReturnsKnownQuirks(t *testing.T) {
	provider, quirks, ok := FromAutoChannelName("[CCT Auto] routeway-001122ff")
	if !ok {
		t.Fatal("expected routeway quirks from auto channel name")
	}
	if provider != "routeway" {
		t.Fatalf("expected provider routeway, got %q", provider)
	}
	if quirks.DefaultUserAgent != "cctapi-free-pool/1.0" {
		t.Fatalf("expected routeway user-agent quirk, got %+v", quirks)
	}
}

func TestFromAutoChannelNameRejectsInvalidSuffix(t *testing.T) {
	if _, _, ok := FromAutoChannelName("[CCT Auto] aihorde-custom-model"); ok {
		t.Fatal("expected custom suffix to be rejected")
	}
}
