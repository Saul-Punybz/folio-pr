package intelligence

// ChatRequest is the input for an AI chat about news.
type ChatRequest struct {
	Question    string
	MaxArticles int    // default 15
	Model       string // default "llama3.2:3b"
}

// LocalSource is a reference to a locally stored article.
type LocalSource struct {
	Title  string `json:"title"`
	Source string `json:"source"`
	URL    string `json:"url"`
}

// WebSource is a reference to a web search result.
type WebSource struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Savable bool   `json:"savable"`
}

// ChatResponse is the output from an AI chat about news.
type ChatResponse struct {
	Answer       string        `json:"answer"`
	ArticlesUsed int           `json:"articles_used"`
	Sources      []LocalSource `json:"sources"`
	WebSources   []WebSource   `json:"web_sources"`
}
