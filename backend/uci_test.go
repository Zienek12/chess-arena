package main

import "testing"

func TestEngineErrorOnlyMatchesErrorPrefix(t *testing.T) {
	if _, ok := engineError("info string NNUE evaluation enabled"); ok {
		t.Fatal("plain info string must not be treated as an error")
	}
	if _, ok := engineError("readyok"); ok {
		t.Fatal("readyok must not be treated as an error")
	}
	err, ok := engineError("info string error illegal move e2e5")
	if !ok {
		t.Fatal("expected error line to be detected")
	}
	if err.Error() != "engine: illegal move e2e5" {
		t.Fatalf("unexpected error message %q", err.Error())
	}
}
