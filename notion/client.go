package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	Token      string
	DatabaseID string
	HTTPClient *http.Client
}

func NewClient(token, databaseID string) *Client {
	return &Client{
		Token:      token,
		DatabaseID: databaseID,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) request(method, url string, body interface{}) ([]byte, error) {
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

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

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
		return nil, fmt.Errorf("notion api error: %d %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) ClearDatabase() error {
	// Query all pages
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.DatabaseID)
	respBody, err := c.request("POST", url, map[string]interface{}{})
	if err != nil {
		return err
	}

	var result struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	// Archive each page
	for _, page := range result.Results {
		archiveURL := fmt.Sprintf("https://api.notion.com/v1/pages/%s", page.ID)
		_, err := c.request("PATCH", archiveURL, map[string]interface{}{
			"archived": true,
		})
		if err != nil {
			fmt.Printf("Warning: failed to archive page %s: %v\n", page.ID, err)
		}
	}

	return nil
}

func (c *Client) AddEntry(username, discordID string) error {
	url := "https://api.notion.com/v1/pages"
	body := map[string]interface{}{
		"parent": map[string]string{"database_id": c.DatabaseID},
		"properties": map[string]interface{}{
			"Name": map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]string{"content": username}},
				},
			},
			"Discord ID": map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]string{"content": discordID}},
				},
			},
			"has_scrolls": map[string]interface{}{
				"checkbox": true,
			},
			"note": map[string]interface{}{
				"number": nil,
			},
		},
	}

	_, err := c.request("POST", url, body)
	return err
}

func (c *Client) GetUsersWithScrolls() (map[string]bool, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.DatabaseID)
	// Filter for has_scrolls == true
	filter := map[string]interface{}{
		"filter": map[string]interface{}{
			"property": "has_scrolls",
			"checkbox": map[string]bool{"equals": true},
		},
	}

	respBody, err := c.request("POST", url, filter)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Properties map[string]interface{} `json:"properties"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	users := make(map[string]bool)
	for _, page := range result.Results {
		// Extract Discord ID
		if prop, ok := page.Properties["Discord ID"].(map[string]interface{}); ok {
			if rt, ok := prop["rich_text"].([]interface{}); ok && len(rt) > 0 {
				if textObj, ok := rt[0].(map[string]interface{}); ok {
					if text, ok := textObj["text"].(map[string]interface{}); ok {
						if content, ok := text["content"].(string); ok {
							users[content] = true
						}
					}
				}
			}
		}
	}

	return users, nil
}
