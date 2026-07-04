package relaymode

import "testing"

func TestGetByPathResponses(t *testing.T) {
	if got := GetByPath("/v1/responses"); got != Responses {
		t.Fatalf("GetByPath(/v1/responses) = %d, want %d", got, Responses)
	}
}
