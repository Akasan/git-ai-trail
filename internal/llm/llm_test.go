package llm

import (
	"testing"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantPass bool
		wantReason string
	}{
		{
			name: "simple pass",
			response: `VERDICT: PASS
REASON: The answer is correct`,
			wantPass: true,
			wantReason: "The answer is correct",
		},
		{
			name: "simple fail",
			response: `VERDICT: FAIL
REASON: The answer is incorrect`,
			wantPass: false,
			wantReason: "The answer is incorrect",
		},
		{
			name: "markdown bold verdict pass",
			response: `**VERDICT:** PASS
**REASON:** Good understanding`,
			wantPass: true,
			wantReason: "Good understanding",
		},
		{
			name: "markdown bold verdict fail",
			response: `**VERDICT:** FAIL
**REASON:** Needs improvement`,
			wantPass: false,
			wantReason: "Needs improvement",
		},
		{
			name: "markdown bold value pass",
			response: `VERDICT: **PASS**
REASON: Excellent work`,
			wantPass: true,
			wantReason: "Excellent work",
		},
		{
			name: "markdown bold value fail",
			response: `VERDICT: **FAIL**
REASON: Try again`,
			wantPass: false,
			wantReason: "Try again",
		},
		{
			name: "markdown both bold pass",
			response: `**VERDICT: PASS**
REASON: Well done`,
			wantPass: true,
			wantReason: "Well done",
		},
		{
			name: "trailing period pass",
			response: `VERDICT: PASS.
REASON: Correct answer.`,
			wantPass: true,
			wantReason: "Correct answer.",
		},
		{
			name: "trailing period fail",
			response: `VERDICT: FAIL.
REASON: Incorrect answer.`,
			wantPass: false,
			wantReason: "Incorrect answer.",
		},
		{
			name: "reason contains verdict keyword",
			response: `VERDICT: FAIL
REASON: The code would fail because it assumes VERDICT: PASS in all cases`,
			wantPass: false,
			wantReason: "The code would fail because it assumes VERDICT: PASS in all cases",
		},
		{
			name: "extra whitespace",
			response: `   VERDICT:   PASS   
   REASON:   Good job   `,
			wantPass: true,
			wantReason: "Good job",
		},
		{
			name: "no reason provided",
			response: `VERDICT: PASS`,
			wantPass: true,
			wantReason: "No reason provided",
		},
		{
			name: "verdict after other content",
			response: `Let me evaluate your answer.

VERDICT: PASS
REASON: You demonstrated understanding`,
			wantPass: true,
			wantReason: "You demonstrated understanding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPass, gotReason := parseVerdict(tt.response)
			if gotPass != tt.wantPass {
				t.Errorf("parseVerdict() pass = %v, want %v", gotPass, tt.wantPass)
			}
			if gotReason != tt.wantReason {
				t.Errorf("parseVerdict() reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}
