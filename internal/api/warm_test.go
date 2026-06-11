package api

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewWarmRequestUsesMatchingRandomReply(t *testing.T) {
	req := newWarmRequest()

	if len(req.Input) != 1 || len(req.Input[0].Content) != 1 {
		t.Fatalf("unexpected warm input shape: %#v", req.Input)
	}

	reply := strings.TrimPrefix(req.Instructions, "Reply exactly: ")
	if reply == req.Instructions || reply == "" {
		t.Fatalf("instructions did not contain reply target: %q", req.Instructions)
	}

	wantInput := "Reply with exactly: " + reply
	if req.Input[0].Content[0].Text != wantInput {
		t.Fatalf("input text = %q, want %q", req.Input[0].Content[0].Text, wantInput)
	}

	if !regexp.MustCompile(`^ok-[0-9a-f]{6}$`).MatchString(reply) {
		t.Fatalf("reply = %q, want ok- plus 6 lowercase hex chars", reply)
	}
}

func TestNewWarmRequestVariesReply(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		reply := strings.TrimPrefix(newWarmRequest().Instructions, "Reply exactly: ")
		seen[reply] = true
	}

	if len(seen) < 2 {
		t.Fatalf("expected randomized warm replies, got %v", seen)
	}
}
