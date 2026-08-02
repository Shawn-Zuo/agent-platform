package mcpclient

import (
	"strings"
	"testing"
)

func TestPublicToolName(t *testing.T) {
	if got, want := publicToolName("my server", "search.web"), "my_server__search_web"; got != want {
		t.Fatalf("publicToolName = %q, want %q", got, want)
	}
	got := publicToolName(strings.Repeat("server", 20), strings.Repeat("tool", 20))
	if len(got) != maxToolNameLength {
		t.Fatalf("long public tool name length = %d, want %d", len(got), maxToolNameLength)
	}
}
