package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ayus-10/zeroku/internal/api"
	"github.com/ayus-10/zeroku/internal/state"
)

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

func (c *Client) Deploy(appName string, req api.DeployRequest) error {
	resp, err := c.do(http.MethodPost, "/v1/apps/"+url.PathEscape(appName)+"/deploy", req)
	if err != nil {
		return fmt.Errorf("deploy: %w", err)
	}
	defer resp.Body.Close()

	var lastLine string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(lastLine)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream: %w", err)
	}

	if msg, ok := strings.CutPrefix(lastLine, "ERROR:"); ok {
		return fmt.Errorf("deploy failed: %s", strings.TrimSpace(msg))
	}
	return nil
}

func (c *Client) GetApp(appName string) (*state.AppDetails, error) {
	var data state.AppDetails
	if err := c.doJSON(http.MethodGet, "/v1/apps/"+url.PathEscape(appName), nil, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) ListApps() ([]state.AppSummary, error) {
	var data []state.AppSummary
	if err := c.doJSON(http.MethodGet, "/v1/apps", nil, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) Stop(appName string) error {
	resp, err := c.do(http.MethodPost, "/v1/apps/"+url.PathEscape(appName)+"/stop", nil)
	if err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	defer resp.Body.Close()

	var lastLine string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(lastLine)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream: %w", err)
	}

	if msg, ok := strings.CutPrefix(lastLine, "ERROR:"); ok {
		return fmt.Errorf("deploy failed: %s", strings.TrimSpace(msg))
	}
	return nil
}

func (c *Client) do(method, path string, in any, okStatus ...int) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send %s %s: %w", method, path, err)
	}

	if len(okStatus) == 0 {
		okStatus = []int{http.StatusOK}
	}
	if !slices.Contains(okStatus, resp.StatusCode) {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s %s failed (%d): %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

func (c *Client) doJSON(method, path string, in, out any, okStatus ...int) error {
	resp, err := c.do(method, path, in, okStatus...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}
