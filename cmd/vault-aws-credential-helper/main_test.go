package main

import "testing"

func TestRun_NoSubcommand(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	if code := run([]string{"bogus"}); code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}
