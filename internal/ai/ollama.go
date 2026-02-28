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
	generateTimeout  = 60 * time.Second
	embeddingTimeout = 30 * time.Second
)

// OllamaClient is an HTTP client for the Ollama API.
type OllamaClient struct {
	baseURL       string
	instructModel string
	embedModel    string
	httpClient    *http.Client
}

// NewClient creates a new OllamaClient configured with the given base URL and
// model names.
func NewClient(baseURL, instructModel, embedModel string) *OllamaClient {
	return &OllamaClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		instructModel: instructModel,
		embedModel:    embedModel,
		httpClient: &http.Client{
			Timeout: generateTimeout,
		},
	}
}

// generateRequest is the JSON body sent to POST /api/generate.
type generateRequest struct {
	Model  string `json:"model"`
	System string `json:"system,omitempty"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// generateResponse is a single JSON object in the Ollama streaming response.
// When stream=false, there is one response with done=true containing the full
// response. When stream=true, each line is a JSON object with a partial
// "response" field.
type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// embeddingRequest is the JSON body sent to POST /api/embeddings.
type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embeddingResponse is the JSON response from POST /api/embeddings.
type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
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

// ExtractEntities asks the LLM to extract key people, organizations, and places.
func (c *OllamaClient) ExtractEntities(ctx context.Context, text string) ([]string, error) {
	systemPrompt := `Extract key people, organizations, and places from this article.

RULES:
- Output ONLY a comma-separated list of names (e.g. "Juan García, Senado de PR, San Juan")
- Do NOT explain your reasoning
- Do NOT add commentary or descriptions
- If no entities found, output "none"`

	resp, err := c.generate(ctx, systemPrompt, text)
	if err != nil {
		return nil, err
	}

	result := parseCSV(resp)
	// Filter out "none" placeholder.
	filtered := make([]string, 0, len(result))
	for _, e := range result {
		if strings.ToLower(e) != "none" {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// Embed generates a vector embedding for the given text using the embedding
// model.
func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embeddingTimeout)
	defer cancel()

	reqBody := embeddingRequest{
		Model:  c.embedModel,
		Prompt: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ollama embed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed: decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embed: empty embedding returned")
	}

	return result.Embedding, nil
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

// generate performs a POST to /api/generate using the default instructModel.
func (c *OllamaClient) generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.generateWithModel(ctx, c.instructModel, systemPrompt, userPrompt)
}

// generateWithModel performs a POST to /api/generate with a specific model and
// concatenates the streamed response into a single string.
func (c *OllamaClient) generateWithModel(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
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
		return "", fmt.Errorf("ollama generate: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama generate: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ollama generate: status %d: %s", resp.StatusCode, string(respBody))
	}

	// Ollama streams JSON objects, one per line. Concatenate the "response"
	// fields to build the full response.
	var sb strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var chunk generateResponse
		if err := decoder.Decode(&chunk); err != nil {
			// If we already have some content, return what we have.
			if sb.Len() > 0 {
				break
			}
			return "", fmt.Errorf("ollama generate: decode chunk: %w", err)
		}
		sb.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("ollama generate: empty response")
	}

	return result, nil
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
