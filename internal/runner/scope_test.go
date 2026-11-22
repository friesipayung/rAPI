package runner

import "testing"

func TestNewScope(t *testing.T) {
	s := NewScope()
	if s == nil {
		t.Fatal("NewScope returned nil")
	}
	if s.Vars == nil || len(s.Vars) != 0 {
		t.Fatalf("Vars must start as an empty non-nil map, got %#v", s.Vars)
	}
	if s.Console != nil {
		t.Fatalf("Console must start nil, got %#v", s.Console)
	}
	if s.Jar != nil {
		t.Fatal("Jar must default to nil (shared jar arrives in a later unit)")
	}
	if s.StopOnFailure {
		t.Fatal("StopOnFailure must default to false")
	}
}
