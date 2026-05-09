package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

func (c *Client) CreateMessage(channelID string, content string, replyToMessageID string, fileName string, fileBytes []byte) (*Message, error) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)

	payload := map[string]interface{}{
		"content": content,
	}
	if replyToMessageID != "" {
		payload["message_reference"] = map[string]string{
			"message_id": replyToMessageID,
		}
	}

	var respBody []byte
	var err error

	if len(fileBytes) > 0 {
		// Multipart request for file upload
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add JSON payload
		jsonPart, err := writer.CreateFormField("payload_json")
		if err != nil {
			return nil, err
		}
		jsonBytes, _ := json.Marshal(payload)
		jsonPart.Write(jsonBytes)

		// Add file
		filePart, err := writer.CreateFormFile("files[0]", fileName)
		if err != nil {
			return nil, err
		}
		filePart.Write(fileBytes)
		writer.Close()

		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bot "+c.Token)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		respBody, _ = io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("discord api error: %d %s", resp.StatusCode, string(respBody))
		}
	} else {
		// Simple JSON request
		respBody, err = c.request("POST", url, payload, true)
		if err != nil {
			return nil, err
		}
	}

	var msg Message
	if err := json.Unmarshal(respBody, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *Client) CreateThread(channelID, messageID, name string) (*Message, error) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/threads", channelID, messageID)
	body := map[string]interface{}{
		"name":                name,
		"auto_archive_duration": 1440,
	}

	respBody, err := c.request("POST", url, body, true)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(respBody, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *Client) AddReaction(channelID, messageID, emoji string) error {
	encodedEmoji := url.PathEscape(emoji)
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, encodedEmoji)
	_, err := c.request("PUT", url, nil, true)
	return err
}

type Channel struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
}

func (c *Client) GetChannel(channelID string) (*Channel, error) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s", channelID)
	respBody, err := c.request("GET", url, nil, true)
	if err != nil {
		return nil, err
	}

	var channel Channel
	if err := json.Unmarshal(respBody, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}
