package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
		model = "claude-sonnet-4-5"
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

	text := apiResp.Content[0].Text
	if text == "" {
		return "", fmt.Errorf("empty content in response")
	}

	return text, nil
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
	lines := strings.Split(response, "\n")
	verdictFound := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "*")
		line = strings.TrimSpace(line)
		
		if !verdictFound && strings.HasPrefix(line, "VERDICT:") {
			verdictPart := strings.TrimPrefix(line, "VERDICT:")
			verdictPart = strings.TrimSpace(verdictPart)
			verdictPart = strings.Trim(verdictPart, "*")
			verdictPart = strings.TrimSpace(verdictPart)
			verdictPart = strings.TrimRight(verdictPart, ".")
			verdictPart = strings.TrimSpace(verdictPart)
			
			pass = verdictPart == "PASS"
			verdictFound = true
		} else if verdictFound && strings.HasPrefix(line, "REASON:") {
			reason = strings.TrimPrefix(line, "REASON:")
			reason = strings.TrimSpace(reason)
			reason = strings.Trim(reason, "*")
			reason = strings.TrimSpace(reason)
			break
		}
	}
	
	if reason == "" {
		reason = "No reason provided"
	}
	
	return pass, reason
}

func GetAPIKey(provider string) (string, error) {
	var envVar string
	switch provider {
	case "anthropic":
		envVar = "ANTHROPIC_API_KEY"
	case "openai":
		envVar = "OPENAI_API_KEY"
	case "xai":
		envVar = "XAI_API_KEY"
	default:
		return "", fmt.Errorf("unknown provider: %s (supported: anthropic, openai, xai)", provider)
	}
	
	apiKey := os.Getenv(envVar)
	if apiKey == "" {
		return "", fmt.Errorf("%s environment variable not set", envVar)
	}
	return apiKey, nil
}

type OpenAIClient struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	if model == "" {
		model = "gpt-4o"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Error   *openaiError   `json:"error,omitempty"`
}

type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

type openaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (c *OpenAIClient) callAPI(prompt string) (string, error) {
	reqBody := openaiRequest{
		Model: c.Model,
		Messages: []openaiMessage{
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

	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

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

	var apiResp openaiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty content in response")
	}

	return content, nil
}

func (c *OpenAIClient) GenerateQuestion(diff string) (string, error) {
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

func (c *OpenAIClient) GradeAnswer(diff string, question string, answer string) (bool, string, error) {
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

type XAIClient struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewXAIClient(apiKey, model, baseURL string) *XAIClient {
	if model == "" {
		model = "grok-2-latest"
	}
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	return &XAIClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *XAIClient) callAPI(prompt string) (string, error) {
	reqBody := openaiRequest{
		Model: c.Model,
		Messages: []openaiMessage{
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

	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

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
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, bodyPreview)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty content in response")
	}

	return content, nil
}

func (c *XAIClient) GenerateQuestion(diff string) (string, error) {
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

func (c *XAIClient) GradeAnswer(diff string, question string, answer string) (bool, string, error) {
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

func NewClient(provider, model, baseURL string) (Client, error) {
	apiKey, err := GetAPIKey(provider)
	if err != nil {
		return nil, err
	}

	switch provider {
	case "anthropic":
		return NewAnthropicClient(apiKey, model), nil
	case "openai":
		return NewOpenAIClient(apiKey, model, baseURL), nil
	case "xai":
		return NewXAIClient(apiKey, model, baseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: anthropic, openai, xai)", provider)
	}
}

type FakeClient struct {
	Questions   map[string]string
	Grades      map[string]bool
	FailMarkers []string
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		Questions:   make(map[string]string),
		Grades:      make(map[string]bool),
		FailMarkers: []string{"banana", "spaceship", "purple"},
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
	
	answerLower := strings.ToLower(answer)
	for _, marker := range f.FailMarkers {
		if strings.Contains(answerLower, marker) {
			return false, fmt.Sprintf("Answer contains fail marker '%s'", marker), nil
		}
	}
	
	return true, "Answer demonstrates sufficient understanding", nil
}
