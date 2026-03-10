// Package ai provides clients for AI/LLM services used in the Folio pipeline.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	generateTimeout  = 120 * time.Second
	embeddingTimeout = 30 * time.Second
)

// OllamaClient is an HTTP client that supports both the Ollama API and
// OpenAI-compatible APIs (OpenAI, Groq, Together, OpenRouter, etc.).
//
// Set AI_PROVIDER=openai and AI_API_KEY=... to use cloud providers.
type OllamaClient struct {
	baseURL       string
	apiKey        string // for OpenAI-compatible providers
	protocol      string // "ollama" or "openai"
	instructModel string
	embedModel    string
	httpClient    *http.Client
}

// NewClient creates a new AI client with the Ollama protocol.
func NewClient(baseURL, instructModel, embedModel string) *OllamaClient {
	return &OllamaClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		protocol:      "ollama",
		instructModel: instructModel,
		embedModel:    embedModel,
		httpClient: &http.Client{
			Timeout: generateTimeout,
		},
	}
}

// NewFromConfig creates the appropriate AI client based on the provider string.
// provider="ollama" uses Ollama, provider="openai" uses OpenAI-compatible API.
func NewFromConfig(provider, host, apiKey, instructModel, embedModel string) *OllamaClient {
	if provider == "openai" {
		return NewOpenAIClient(host, apiKey, instructModel, embedModel)
	}
	return NewClient(host, instructModel, embedModel)
}

// NewOpenAIClient creates a new AI client using the OpenAI-compatible API.
// Works with: OpenAI, Groq, Together, OpenRouter, Mistral, any OpenAI-compatible provider.
//
// Example base URLs:
//
//	OpenAI:      https://api.openai.com
//	Groq:        https://api.groq.com/openai
//	Together:    https://api.together.xyz
//	OpenRouter:  https://openrouter.ai/api
//	Mistral:     https://api.mistral.ai
func NewOpenAIClient(baseURL, apiKey, instructModel, embedModel string) *OllamaClient {
	return &OllamaClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		apiKey:        apiKey,
		protocol:      "openai",
		instructModel: instructModel,
		embedModel:    embedModel,
		httpClient: &http.Client{
			Timeout: generateTimeout,
		},
	}
}

// ── Ollama protocol types ────────────────────────────────────

type generateRequest struct {
	Model  string `json:"model"`
	System string `json:"system,omitempty"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// ── OpenAI protocol types ────────────────────────────────────

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type openaiEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openaiEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Summarize asks the LLM to produce a 2-3 sentence summary of the given text.
func (c *OllamaClient) Summarize(ctx context.Context, text string) (string, error) {
	systemPrompt := `You are a news summarizer. Your ONLY job is to output a 2-3 sentence summary.

RULES:
- Write the summary in the SAME language as the article
- Be factual and concise
- Output ONLY the summary text, nothing else
- Do NOT explain what you are doing
- Do NOT say "I cannot" or "there is no information"
- Do NOT add commentary, disclaimers, or meta-text
- If the text is short, summarize what is there`

	summary, err := c.generate(ctx, systemPrompt, text)
	if err != nil {
		return "", err
	}

	// Validate: reject responses that look like AI commentary instead of summaries.
	summary = cleanAIResponse(summary)
	if summary == "" {
		return "", fmt.Errorf("ollama summarize: produced empty or invalid summary")
	}
	return summary, nil
}

// Classify asks the LLM to assign 1-3 topic tags from a fixed taxonomy.
func (c *OllamaClient) Classify(ctx context.Context, text string) ([]string, error) {
	systemPrompt := `You are a strict tag classifier. You receive article text and output ONLY comma-separated tags.

ALLOWED TAGS: politics, economy, health, education, infrastructure, environment, crime, grants, federal, legislation, government, technology, culture, sports

EXAMPLES:
Article about governor signing a bill → politics, legislation
Article about hospital funding cuts → health, economy
Article about federal grants for schools → grants, education, federal
Article about road construction project → infrastructure
Article about arrests in Bayamón → crime
Article about tech startup in San Juan → technology, economy

RULES:
- Output ONLY tags from the list above, comma-separated
- Pick 1-3 tags that best fit
- NO explanations, NO sentences, NO commentary
- NEVER output anything except tag names separated by commas
- If unsure, pick the closest match`

	resp, err := c.generate(ctx, systemPrompt, text)
	if err != nil {
		return nil, err
	}

	// Clean and validate tags against the allowed set.
	return parseAndValidateTags(resp), nil
}

// ExtractedEntities holds categorized entities extracted from article text by
// the LLM. Each field contains entity names of the corresponding type.
type ExtractedEntities struct {
	People        []string `json:"people"`
	Organizations []string `json:"organizations"`
	Places        []string `json:"places"`
}

// ExtractEntities asks the LLM to extract key people, organizations, and places
// from article text, returning them in categorized form.
func (c *OllamaClient) ExtractEntities(ctx context.Context, text string) (*ExtractedEntities, error) {
	systemPrompt := `Extract key entities from this article text. Return a JSON object with these keys:
- "people": array of person names mentioned
- "organizations": array of organization/company/agency names
- "places": array of geographic locations

RULES:
- Output ONLY valid JSON, nothing else
- Each array can be empty if no entities of that type are found
- Use the original names as they appear in the text
- Do NOT include descriptions or explanations

Example: {"people": ["Juan García", "María López"], "organizations": ["Senado de PR"], "places": ["San Juan"]}`

	resp, err := c.generate(ctx, systemPrompt, text)
	if err != nil {
		return nil, err
	}

	// Try to parse the JSON response.
	var result ExtractedEntities
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp)), &result); err != nil {
		// If the response contains JSON embedded in other text, try to extract it.
		if start := strings.Index(resp, "{"); start != -1 {
			if end := strings.LastIndex(resp, "}"); end > start {
				if err2 := json.Unmarshal([]byte(resp[start:end+1]), &result); err2 == nil {
					return &result, nil
				}
			}
		}
		// Fall back to empty entities on parse failure.
		return &ExtractedEntities{}, nil
	}
	return &result, nil
}

// ClassifySentiment asks the LLM to classify the overall sentiment of the text
// as "positive", "neutral", or "negative". Returns "neutral" if parsing fails.
func (c *OllamaClient) ClassifySentiment(ctx context.Context, text string) (string, error) {
	systemPrompt := `Classify the overall sentiment of this article text.

RULES:
- Output ONLY one word: "positive", "neutral", or "negative"
- Do NOT explain your reasoning
- Do NOT add any other text
- If unsure, output "neutral"`

	resp, err := c.generate(ctx, systemPrompt, text)
	if err != nil {
		return "neutral", err
	}

	// Normalize the response.
	sentiment := strings.ToLower(strings.TrimSpace(resp))

	// Accept only valid sentiment values.
	switch sentiment {
	case "positive", "neutral", "negative":
		return sentiment, nil
	default:
		// Try to find a valid sentiment within the response.
		for _, valid := range []string{"positive", "negative", "neutral"} {
			if strings.Contains(sentiment, valid) {
				return valid, nil
			}
		}
		return "neutral", nil
	}
}

// Embed generates a vector embedding for the given text using the embedding model.
func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.protocol == "openai" {
		return c.embedOpenAI(ctx, text)
	}
	return c.embedOllama(ctx, text)
}

func (c *OllamaClient) embedOllama(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embeddingTimeout)
	defer cancel()

	reqBody := embeddingRequest{
		Model:  c.embedModel,
		Prompt: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("embed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("embed: empty embedding returned")
	}

	return result.Embedding, nil
}

func (c *OllamaClient) embedOpenAI(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embeddingTimeout)
	defer cancel()

	reqBody := openaiEmbedRequest{
		Model: c.embedModel,
		Input: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("embed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openaiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("embed: API error: %s", result.Error.Message)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embed: empty embedding returned")
	}

	return result.Data[0].Embedding, nil
}

// Generate performs an LLM generation with a custom system prompt and user prompt.
func (c *OllamaClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.generate(ctx, systemPrompt, userPrompt)
}

// GenerateWithModel performs an LLM generation using a specific model override.
// Use this when you need a different model than the default instructModel (e.g.
// a faster model for interactive chat vs a quality model for batch processing).
func (c *OllamaClient) GenerateWithModel(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	return c.generateWithModel(ctx, model, systemPrompt, userPrompt)
}

// generate performs text generation using the default instructModel.
func (c *OllamaClient) generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.generateWithModel(ctx, c.instructModel, systemPrompt, userPrompt)
}

// generateWithModel performs text generation with a specific model.
// Routes to either Ollama or OpenAI protocol based on client configuration.
func (c *OllamaClient) generateWithModel(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	if c.protocol == "openai" {
		return c.generateOpenAI(ctx, model, systemPrompt, userPrompt)
	}
	return c.generateOllama(ctx, model, systemPrompt, userPrompt)
}

// generateOllama uses the native Ollama API (POST /api/generate).
func (c *OllamaClient) generateOllama(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()

	reqBody := generateRequest{
		Model:  model,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("generate: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("generate: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("generate: status %d: %s", resp.StatusCode, string(respBody))
	}

	var sb strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var chunk generateResponse
		if err := decoder.Decode(&chunk); err != nil {
			if sb.Len() > 0 {
				break
			}
			return "", fmt.Errorf("generate: decode chunk: %w", err)
		}
		sb.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("generate: empty response")
	}

	return result, nil
}

// generateOpenAI uses the OpenAI chat completions API (POST /v1/chat/completions).
// Works with OpenAI, Groq, Together, OpenRouter, Mistral, and any compatible provider.
func (c *OllamaClient) generateOpenAI(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()

	messages := []openaiMessage{
		{Role: "user", Content: userPrompt},
	}
	if systemPrompt != "" {
		messages = append([]openaiMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	reqBody := openaiChatRequest{
		Model:    model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("generate: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("generate: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("generate: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("generate: decode response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("generate: API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("generate: empty response (no choices)")
	}

	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("generate: empty response")
	}

	return text, nil
}

// allowedTags is the set of valid topic tags for classification.
var allowedTags = map[string]bool{
	"politics": true, "economy": true, "health": true, "education": true,
	"infrastructure": true, "environment": true, "crime": true, "grants": true,
	"federal": true, "legislation": true, "government": true, "technology": true,
	"culture": true, "sports": true,
}

// garbagePatterns are substrings that indicate the AI returned commentary instead
// of the requested output. Case-insensitive check.
var garbagePatterns = []string{
	"no hay información",
	"no tengo",
	"no puedo",
	"no hay suficiente",
	"i cannot",
	"i don't have",
	"there is no information",
	"none of the provided",
	"i can suggest",
	"puedo sugerir",
	"sin embargo",
	"however",
	"por favor proporciona",
	"please provide",
	"no me permite",
	"clasificarlo en",
	"no information about",
	"que son:",
	"they might fit",
	"if i had to",
	"based on the context",
	"basada en",
	"posibles etiquetas",
	"si deseas más",
}

// cleanAIResponse strips garbage AI commentary from a response. Returns empty
// string if the response is entirely garbage.
func cleanAIResponse(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	lower := strings.ToLower(s)
	for _, pattern := range garbagePatterns {
		if strings.Contains(lower, pattern) {
			return ""
		}
	}

	// Strip leading/trailing quotes.
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}

// parseAndValidateTags parses a CSV response and filters to only allowed tags.
func parseAndValidateTags(s string) []string {
	raw := parseCSV(s)
	var valid []string
	for _, tag := range raw {
		t := strings.ToLower(strings.TrimSpace(tag))
		// Strip surrounding noise like "1." or "-"
		t = strings.TrimLeft(t, "0123456789.- ")
		if allowedTags[t] {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		// If all tags were rejected, try to salvage by matching substrings.
		for _, tag := range raw {
			t := strings.ToLower(tag)
			for allowed := range allowedTags {
				if strings.Contains(t, allowed) {
					valid = append(valid, allowed)
					break
				}
			}
		}
	}
	// Deduplicate.
	seen := make(map[string]bool)
	var deduped []string
	for _, t := range valid {
		if !seen[t] {
			seen[t] = true
			deduped = append(deduped, t)
		}
	}
	return deduped
}

// parseCSV splits a comma-separated string into trimmed, non-empty tokens.
func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Strip surrounding quotes if present.
		p = strings.Trim(p, `"'`)
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, strings.ToLower(p))
		}
	}
	return result
}
