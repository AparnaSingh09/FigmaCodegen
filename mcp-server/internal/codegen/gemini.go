package codegen

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// GeminiGenerator implements CodeGenerator using Google's Gemini API.
// Gemini was picked as the first provider because it has a genuinely free
// tier with real vision support - this task needs the model to look at
// the rendered frame, not just read text.
type GeminiGenerator struct {
	client *genai.Client
	model  string
}

func NewGeminiGenerator(ctx context.Context, apiKey, model string) (*GeminiGenerator, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating gemini client: %w", err)
	}
	return &GeminiGenerator{client: client, model: model}, nil
}

func (g *GeminiGenerator) GenerateCode(ctx context.Context, image []byte, nodeJSON, framework string) (string, error) {
	prompt := fmt.Sprintf(
		"You are a frontend engineer. Generate clean, working %s code that reproduces "+
			"the attached UI design as closely as possible - layout, spacing, text, and colors. "+
			"Use the structured design data below only as supporting context (it may be partial); "+
			"the image is the source of truth for how it should look. "+
			"Return ONLY the code, no explanation.\n\nStructured design data:\n%s",
		framework, nodeJSON,
	)

	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: image, MIMEType: "image/png"}},
	}
	contents := []*genai.Content{{Parts: parts}}

	result, err := g.client.Models.GenerateContent(ctx, g.model, contents, nil)
	if err != nil {
		return "", fmt.Errorf("gemini generate content: %w", err)
	}

	code := result.Text()
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("gemini returned an empty response")
	}
	return code, nil
}
