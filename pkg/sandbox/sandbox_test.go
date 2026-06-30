package sandbox

import "testing"

func TestExecResultCombined(t *testing.T) {
	cases := []struct {
		name           string
		stdout, stderr string
		want           string
	}{
		{"both", "out", "err", "out\nerr"},
		{"stdout only", "out", "", "out"},
		{"stderr only", "", "err", "err"},
		{"neither", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := ExecResult{Stdout: c.stdout, Stderr: c.stderr}
			if got := r.Combined(); got != c.want {
				t.Fatalf("Combined() = %q, want %q", got, c.want)
			}
		})
	}
}
