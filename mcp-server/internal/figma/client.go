// Package figma wraps the parts of Figma's REST API this project needs:
// turning a pasted Figma link into a file key + node ID, fetching that
// node's structured data, and fetching a rendered PNG of it.
//
// Note: Figma's REST API authenticates personal access tokens with a
// custom "X-Figma-Token" header - NOT "Authorization: Bearer" (that header
// is for OAuth tokens only, and silently 403s a personal access token).
package figma

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const baseURL = "https://api.figma.com/v1"

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, httpClient: &http.Client{}}
}

// figmaURLPattern matches both the older /file/ links and the current
// /design/ links Figma issues, e.g.:
//
//	https://www.figma.com/file/ABC123/My-Design?node-id=1-23
//	https://www.figma.com/design/ABC123/My-Design?node-id=1-23
var figmaURLPattern = regexp.MustCompile(`figma\.com/(?:file|design)/([a-zA-Z0-9]+)/`)

// ParseURL extracts the file key and node ID from a Figma share link. The
// node-id query param uses hyphens (e.g. "1-23") but the REST API expects
// a colon (e.g. "1:23") - this converts it.
func ParseURL(figmaURL string) (fileKey, nodeID string, err error) {
	match := figmaURLPattern.FindStringSubmatch(figmaURL)
	if match == nil {
		return "", "", fmt.Errorf("not a recognizable Figma file/design URL: %s", figmaURL)
	}
	fileKey = match[1]

	parsed, err := url.Parse(figmaURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing Figma URL: %w", err)
	}
	rawNodeID := parsed.Query().Get("node-id")
	if rawNodeID == "" {
		return "", "", fmt.Errorf("Figma URL has no node-id query param - select the specific frame in Figma, then \"Copy link to selection\"")
	}
	nodeID = strings.Replace(rawNodeID, "-", ":", 1)

	return fileKey, nodeID, nil
}

// nodesResponse mirrors the relevant shape of GET /v1/files/:key/nodes.
// We don't need every field Figma returns, just enough structured context
// (layout, text, basic styling) to hand the LLM alongside the image.
type nodesResponse struct {
	Nodes map[string]struct {
		Document json.RawMessage `json:"document"`
	} `json:"nodes"`
}

// GetNodeJSON fetches the raw structured document for a single node, as a
// JSON string suitable for including in an LLM prompt alongside the image.
func (c *Client) GetNodeJSON(fileKey, nodeID string) (string, error) {
	endpoint := fmt.Sprintf("%s/files/%s/nodes?ids=%s", baseURL, fileKey, url.QueryEscape(nodeID))

	body, err := c.get(endpoint)
	if err != nil {
		return "", err
	}

	var parsed nodesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing nodes response: %w", err)
	}
	node, ok := parsed.Nodes[nodeID]
	if !ok {
		return "", fmt.Errorf("node %s not found in file %s - check the link points at a real frame", nodeID, fileKey)
	}
	return string(node.Document), nil
}

// imagesResponse mirrors GET /v1/images/:key, which doesn't return image
// bytes directly - it returns a short-lived URL per node ID that you then
// fetch separately.
type imagesResponse struct {
	Images map[string]string `json:"images"`
	Err    *string           `json:"err"`
}

// GetImagePNG renders the given node to PNG and returns the raw image
// bytes, capped at maxBytes (Figma frames are normally tiny, but a huge
// frame shouldn't silently blow up the LLM request).
func (c *Client) GetImagePNG(fileKey, nodeID string, maxBytes int) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/images/%s?ids=%s&format=png", baseURL, fileKey, url.QueryEscape(nodeID))

	body, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}

	var parsed imagesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing images response: %w", err)
	}
	if parsed.Err != nil {
		return nil, fmt.Errorf("figma image render error: %s", *parsed.Err)
	}
	imageURL, ok := parsed.Images[nodeID]
	if !ok || imageURL == "" {
		return nil, fmt.Errorf("figma returned no image for node %s", nodeID)
	}

	// The image itself is served from Figma's asset CDN, unauthenticated -
	// no X-Figma-Token needed for this second request.
	resp, err := c.httpClient.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("fetching rendered image: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading rendered image: %w", err)
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("rendered image exceeds %d byte cap", maxBytes)
	}
	return data, nil
}

func (c *Client) get(endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Figma-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling figma api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading figma response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			return nil, fmt.Errorf("figma api rate limit exceeded - retry after %s seconds", retryAfter)
		}
		return nil, fmt.Errorf("figma api rate limit exceeded - wait a bit and retry: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("figma api returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
