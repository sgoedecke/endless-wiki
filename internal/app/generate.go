package app

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

const groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"
const groqModel = "llama3-70b-8192"

// GeneratePageHTML produces article HTML for a slug, calling Groq when possible.
func GeneratePageHTML(ctx context.Context, client *http.Client, cfg Config, slug string) (string, error) {
	if cfg.GroqAPIKey == "" {
		return stubPage(slug), nil
	}

	payload := groqChatRequest{
		Model: groqModel,
		Messages: []groqMessage{
			{
				Role:    "system",
				Content: "You are composing clean HTML for a fictional encyclopedia. Output only valid HTML with a single <h1> title and a <div class=\"infiniwiki-body\"> wrapping the body. Include 3-6 internal links in the body pointing to related topics using <a href=\"/wiki/...\"> text.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Write a concise Wikipedia-style article about '%s'. Keep to 5 short paragraphs and include an unordered list summarizing key facts.", SlugTitle(slug)),
			},
		},
		Temperature: 0.7,
		MaxTokens:   900,
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqEndpoint, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.GroqAPIKey)

	// ensure client is non-nil
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq error: status %d body %s", resp.StatusCode, truncate(string(body), 512))
	}

	var gr groqChatResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", err
	}

	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("groq response missing choices")
	}

	content := strings.TrimSpace(gr.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("groq response empty")
	}

	return content, nil
}

func stubPage(slug string) string {
	title := SlugTitle(slug)
	links := relatedSlugs(slug)

	var b strings.Builder
	b.WriteString("<h1>")
	b.WriteString(templateEscape(title))
	b.WriteString("</h1>\n<div class=\"infiniwiki-body\">\n")
	b.WriteString("<p>This Infiniwiki entry for ")
	b.WriteString(templateEscape(title))
	b.WriteString(" is a placeholder generated without Groq access. It outlines the topic and suggests related articles.</p>\n")
	b.WriteString("<p>Future iterations will fetch richer AI generated prose from Groq's models.</p>\n")
	b.WriteString("<ul class=\"infiniwiki-summary\">\n")
	for _, link := range links {
		b.WriteString("  <li><a href=\"/wiki/")
		b.WriteString(link)
		b.WriteString("\">")
		b.WriteString(templateEscape(SlugTitle(link)))
		b.WriteString("</a></li>\n")
	}
	b.WriteString("</ul>\n")
	b.WriteString("<p>Use the related links to continue your journey through the infinite encyclopedia.</p>\n")
	b.WriteString("</div>")

	return b.String()
}

func relatedSlugs(slug string) []string {
	base := strings.TrimSuffix(slug, "_overview")
	if base == "" {
		base = slug
	}
	variants := []string{
		base + "_history",
		base + "_applications",
		base + "_controversies",
	}
	for i, v := range variants {
		normalized, err := NormalizeSlug(v)
		if err == nil {
			variants[i] = normalized
		} else {
			variants[i] = slug
		}
	}
	return variants
}

func templateEscape(input string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(input)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

type groqChatRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqChatResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}
