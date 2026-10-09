package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client interface {
	GenerateQuestion(diff string) (string, error)
	GradeAnswer(diff string, question string, answer string) (bool, string, error)
}

type AnthropicClient struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	return &AnthropicClient{
		APIKey: apiKey,
		Model:  model,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type anthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []content `json:"content"`
	Error   *apiError `json:"error,omitempty"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (c *AnthropicClient) callAPI(prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.Model,
		MaxTokens: 1024,
		Messages: []message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return apiResp.Content[0].Text, nil
}

func (c *AnthropicClient) GenerateQuestion(diff string) (string, error) {
	prompt := fmt.Sprintf(`You are reviewing an AI-generated code change. Generate 1-2 short, specific questions that would test whether the developer understands this change.

Focus on:
- What the code does and why it's needed
- Edge cases or potential issues
- How it interacts with the rest of the system

Code diff:
%s

Return only the questions, one per line. Keep questions concise and specific.`, diff)

	return c.callAPI(prompt)
}

func (c *AnthropicClient) GradeAnswer(diff string, question string, answer string) (bool, string, error) {
	prompt := fmt.Sprintf(`You are reviewing whether a developer understands an AI-generated code change.

Code diff:
%s

Question asked: %s

Developer's answer: %s

Does the answer demonstrate understanding of the code change? Consider:
- Does it show they understand what the code does?
- Do they recognize potential issues or edge cases?
- Is the explanation accurate (even if brief)?

Respond in this exact format:
VERDICT: [PASS or FAIL]
REASON: [one sentence explanation]

Be strict but fair. A correct high-level understanding is acceptable even if not detailed.`, diff, question, answer)

	response, err := c.callAPI(prompt)
	if err != nil {
		return false, "", err
	}

	verdict, reason := parseVerdict(response)
	return verdict, reason, nil
}

func parseVerdict(response string) (pass bool, reason string) {
	lines := bytes.Split([]byte(response), []byte("\n"))
	
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("VERDICT:")) {
			verdict := string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("VERDICT:"))))
			pass = verdict == "PASS"
		} else if bytes.HasPrefix(line, []byte("REASON:")) {
			reason = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("REASON:"))))
		}
	}
	
	if reason == "" {
		reason = "No reason provided"
	}
	
	return pass, reason
}

func GetAPIKey() (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	return apiKey, nil
}

type FakeClient struct {
	Questions map[string]string
	Grades    map[string]bool
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		Questions: make(map[string]string),
		Grades:    make(map[string]bool),
	}
}

func (f *FakeClient) GenerateQuestion(diff string) (string, error) {
	if q, ok := f.Questions[diff]; ok {
		return q, nil
	}
	return "What does this code do?", nil
}

func (f *FakeClient) GradeAnswer(diff string, question string, answer string) (bool, string, error) {
	key := diff + question + answer
	if grade, ok := f.Grades[key]; ok {
		if grade {
			return true, "Answer demonstrates understanding", nil
		}
		return false, "Answer does not demonstrate sufficient understanding", nil
	}
	return true, "Default pass", nil
}
