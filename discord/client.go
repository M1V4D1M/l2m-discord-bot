package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	Token      string
	AppID      string
	HTTPClient *http.Client
}

func NewClient(token, appID string) *Client {
	return &Client{
		Token:      token,
		AppID:      appID,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) request(method, url string, body interface{}, useBotToken bool) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if useBotToken {
		req.Header.Set("Authorization", "Bot "+c.Token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("discord api error: %d %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type Message struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Author    User   `json:"author"`
	Attachments []struct {
		ID string `json:"id"`
	} `json:"attachments"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func (c *Client) FetchThreadMessages(channelID string) ([]Message, error) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages?limit=100", channelID)
	respBody, err := c.request("GET", url, nil, true)
	if err != nil {
		return nil, err
	}

	var messages []Message
	if err := json.Unmarshal(respBody, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (c *Client) EditInteractionResponse(token string, content string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s/messages/@original", c.AppID, token)
	body := map[string]interface{}{
		"content": content,
	}

	_, err := c.request("PATCH", url, body, false)
	return err
}

func (c *Client) CreateMessage(channelID string, content string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	body := map[string]interface{}{
		"content": content,
	}

	_, err := c.request("POST", url, body, true)
	return err
}
