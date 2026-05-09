package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	Token      string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *ImageURLPart  `json:"image_url,omitempty"`
}

type ImageURLPart struct {
	URL string `json:"url"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) GetItemNameFromImage(imageBytes []byte) (string, error) {
	base64Image := base64.StdEncoding.EncodeToString(imageBytes)
	
	reqBody := ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: "Image can consist two or more names, give only item name (that written near item image, commonly red or purple color). Result should be JSON pass value in \"name\" att",
					},
					{
						Type: "image_url",
						ImageURL: &ImageURLPart{
							URL: fmt.Sprintf("data:image/png;base64,%s", base64Image),
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("openai api error: %d %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}

	// Parse JSON from content
	var result struct {
		Name string `json:"name"`
	}
	
	content := chatResp.Choices[0].Message.Content
	// GPT sometimes wraps JSON in markdown blocks
	if len(content) > 7 && content[:7] == "```json" {
		content = content[7 : len(content)-3]
	} else if len(content) > 3 && content[:3] == "```" {
		content = content[3 : len(content)-3]
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Fallback: if it's not JSON, just return the content as is
		return content, nil
	}

	return result.Name, nil
}
