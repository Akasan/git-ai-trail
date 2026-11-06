package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		envVar   string
		envValue string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "anthropic with key",
			provider: "anthropic",
			envVar:   "ANTHROPIC_API_KEY",
			envValue: "test-key",
			wantErr:  false,
		},
		{
			name:     "anthropic without key",
			provider: "anthropic",
			envVar:   "ANTHROPIC_API_KEY",
			envValue: "",
			wantErr:  true,
			errMsg:   "ANTHROPIC_API_KEY environment variable not set",
		},
		{
			name:     "openai with key",
			provider: "openai",
			envVar:   "OPENAI_API_KEY",
			envValue: "test-key",
			wantErr:  false,
		},
		{
			name:     "openai without key",
			provider: "openai",
			envVar:   "OPENAI_API_KEY",
			envValue: "",
			wantErr:  true,
			errMsg:   "OPENAI_API_KEY environment variable not set",
		},
		{
			name:     "xai with key",
			provider: "xai",
			envVar:   "XAI_API_KEY",
			envValue: "test-key",
			wantErr:  false,
		},
		{
			name:     "xai without key",
			provider: "xai",
			envVar:   "XAI_API_KEY",
			envValue: "",
			wantErr:  true,
			errMsg:   "XAI_API_KEY environment variable not set",
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			wantErr:  true,
			errMsg:   "unknown provider: unknown (supported: anthropic, openai, xai)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				oldVal := os.Getenv(tt.envVar)
				defer os.Setenv(tt.envVar, oldVal)
				
				if tt.envValue != "" {
					os.Setenv(tt.envVar, tt.envValue)
				} else {
					os.Unsetenv(tt.envVar)
				}
			}

			key, err := GetAPIKey(tt.provider)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetAPIKey() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("GetAPIKey() error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("GetAPIKey() unexpected error: %v", err)
				}
				if key != tt.envValue {
					t.Errorf("GetAPIKey() = %v, want %v", key, tt.envValue)
				}
			}
		})
	}
}

func TestAnthropicClientIntegration(t *testing.T) {
	t.Skip("Skipping Anthropic integration test - client has hardcoded URL. Use OpenAI/xAI tests as reference for provider integration testing.")
}

func TestOpenAIClientIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}
		
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		
		if req.Model != "gpt-4o" {
			t.Errorf("Expected model gpt-4o, got %s", req.Model)
		}
		
		resp := openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Content: "VERDICT: PASS\nREASON: Good answer"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-4o", server.URL)
	
	pass, reason, err := client.GradeAnswer("diff", "question", "answer")
	if err != nil {
		t.Errorf("GradeAnswer() error = %v", err)
	}
	if !pass {
		t.Errorf("Expected PASS, got FAIL")
	}
	if reason != "Good answer" {
		t.Errorf("Expected reason 'Good answer', got %s", reason)
	}
}

func TestXAIClientIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}
		
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		
		if req.Model != "grok-2-latest" {
			t.Errorf("Expected model grok-2-latest, got %s", req.Model)
		}
		
		resp := openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Content: "VERDICT: FAIL\nREASON: Insufficient understanding"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewXAIClient("test-key", "grok-2-latest", server.URL)
	
	pass, reason, err := client.GradeAnswer("diff", "question", "answer")
	if err != nil {
		t.Errorf("GradeAnswer() error = %v", err)
	}
	if pass {
		t.Errorf("Expected FAIL, got PASS")
	}
	if reason != "Insufficient understanding" {
		t.Errorf("Expected reason 'Insufficient understanding', got %s", reason)
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		baseURL  string
		envVar   string
		wantErr  bool
	}{
		{
			name:     "anthropic",
			provider: "anthropic",
			model:    "claude-sonnet-4-5",
			envVar:   "ANTHROPIC_API_KEY",
			wantErr:  false,
		},
		{
			name:     "openai",
			provider: "openai",
			model:    "gpt-4o",
			envVar:   "OPENAI_API_KEY",
			wantErr:  false,
		},
		{
			name:     "xai",
			provider: "xai",
			model:    "grok-2-latest",
			envVar:   "XAI_API_KEY",
			wantErr:  false,
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				oldVal := os.Getenv(tt.envVar)
				defer os.Setenv(tt.envVar, oldVal)
				os.Setenv(tt.envVar, "test-key")
			}

			client, err := NewClient(tt.provider, tt.model, tt.baseURL, "")
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("NewClient() returned nil client")
				}
			}
		})
	}
}

func TestGenerateQuestionLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		want     string
	}{
		{name: "default", language: "", want: "Write the questions in English."},
		{name: "japanese", language: "Japanese", want: "Write the questions in Japanese."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prompt string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openaiRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				prompt = req.Messages[0].Content
				_ = json.NewEncoder(w).Encode(openaiResponse{
					Choices: []openaiChoice{{Message: openaiMessage{Content: "Q?"}}},
				})
			}))
			defer server.Close()

			client := NewOpenAIClient("test-key", "gpt-4o", server.URL)
			client.Language = tt.language

			if _, err := client.GenerateQuestions("diff"); err != nil {
				t.Fatalf("GenerateQuestions() error = %v", err)
			}
			if !strings.Contains(prompt, tt.want) {
				t.Errorf("prompt missing %q:\n%s", tt.want, prompt)
			}
		})
	}
}

func TestParseQuestions(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     []string
	}{
		{"one per line", "What does it do?\nWhy is it needed?", []string{"What does it do?", "Why is it needed?"}},
		{"blank lines", "\nWhat does it do?\n\n  Why?  \n", []string{"What does it do?", "Why?"}},
		{"numbered", "1. What does it do?\n2) Why?", []string{"What does it do?", "Why?"}},
		{"bullets", "- What?\n* Why?\n• How?", []string{"What?", "Why?", "How?"}},
		{"Q prefix", "Q1: What?\nQuestion 2: Why?\nQ: How?", []string{"What?", "Why?", "How?"}},
		{"markdown bold", "**1.** What?\n**Why?**", []string{"What?", "Why?"}},
		{"japanese", "1．この関数は何をしますか？\nQ2：なぜ必要ですか？", []string{"この関数は何をしますか？", "なぜ必要ですか？"}},
		{"leading number kept", "2 goroutines write here; what prevents a race?", []string{"2 goroutines write here; what prevents a race?"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQuestions(tt.response)
			if err != nil {
				t.Fatalf("parseQuestions() error = %v", err)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("parseQuestions() = %q, want %q", got, tt.want)
			}
		})
	}

	if _, err := parseQuestions(" \n\n"); err == nil {
		t.Error("expected error for empty response")
	}
}
