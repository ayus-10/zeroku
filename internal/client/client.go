package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DeployRequest struct {
	Repo          string `json:"repo"`
	GitRef        string `json:"git_ref"`
	ContainerPort int    `json:"container_port"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

func (c *Client) Deploy(appName string, req DeployRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal deploy request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/apps/%s/deploy", c.BaseURL, appName)

	httpReq, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("create deploy request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send deploy request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("deployment failed: API returned %s", resp.Status)
	}

	return nil
}
