package main

import "testing"

func TestHelloWorld(t *testing.T) {
	want := "Hello, world!"
	got := getHelloWorld()
	if got != want {
			t.Errorf("got %q, want %q", got, want)
	}
}
