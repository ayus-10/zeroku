package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ayus-10/zeroku/internal/state"
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploy failed: %s", strings.TrimSpace(string(body)))
	}

	var lastLine string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		lastLine = line
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream: %w", err)
	}

	if strings.HasPrefix(lastLine, "ERROR:") {
		return fmt.Errorf("deploy failed: %s", strings.TrimSpace(lastLine[6:]))
	}

	return nil
}

func (c *Client) GetApp(appName string) (*state.AppDetails, error) {
	url := fmt.Sprintf("%s/v1/apps/%s", c.BaseURL, appName)

	httpReq, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create get app request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send get app request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get app failed: %s", strings.TrimSpace(string(body)))
	}

	var app state.AppDetails
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, fmt.Errorf("decode get app response: %w", err)
	}

	return &app, nil
}

func (c *Client) ListApps() ([]state.AppSummary, error) {
	url := fmt.Sprintf("%s/v1/apps", c.BaseURL)

	httpReq, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create list apps request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send list apps request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list apps failed: %s", strings.TrimSpace(string(body)))
	}

	var apps []state.AppSummary
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, fmt.Errorf("decode list apps response: %w", err)
	}

	return apps, nil
}
