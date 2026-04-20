package ansi

import "testing"

func TestColorizeAppliesColorPerLine(t *testing.T) {
	got := Colorize(LightRed, "one\ntwo")
	want := "\033[91mone\033[0m\n\033[91mtwo\033[0m"

	if got != want {
		t.Fatalf("Colorize() = %q, want %q", got, want)
	}
}
