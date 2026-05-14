package server

import "testing"

func TestCloneBytesReturnsOwnedCopy(t *testing.T) {
	in := []byte("quarkfile")
	out := cloneBytes(in)
	if string(out) != "quarkfile" {
		t.Fatalf("clone = %q", out)
	}
	in[0] = 'Q'
	if string(out) != "quarkfile" {
		t.Fatalf("clone changed after input mutation: %q", out)
	}
}
