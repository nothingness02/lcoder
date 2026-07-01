package contextmgr

import "testing"

func TestResolveMaxTokens(t *testing.T) {
	tests := []struct {
		name   string
		budget TokenBudget
		input  int
		want   int
	}{
		{
			name:   "ceiling binds when window is roomy",
			budget: TokenBudget{MaxTotal: 200000, MaxOutput: 16384},
			input:  5000,
			want:   16384,
		},
		{
			name:   "remaining window binds when nearly full",
			budget: TokenBudget{MaxTotal: 200000, MaxOutput: 16384},
			input:  190000,
			want:   10000, // 200000 - 190000
		},
		{
			name:   "floor applies when window is exhausted",
			budget: TokenBudget{MaxTotal: 200000, MaxOutput: 16384},
			input:  199900,
			want:   budgetMinOutput,
		},
		{
			name:   "unknown ceiling falls back to constant",
			budget: TokenBudget{MaxTotal: 200000, MaxOutput: 0},
			input:  1000,
			want:   budgetFallbackOutput,
		},
		{
			name:   "no window means ceiling only",
			budget: TokenBudget{MaxTotal: 0, MaxOutput: 8000},
			input:  999999,
			want:   8000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.budget.ResolveMaxTokens(tt.input); got != tt.want {
				t.Fatalf("ResolveMaxTokens(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
