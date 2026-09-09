package textapi

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

const cdpBaseURL = "https://api.text.com/cdp"

type CDPClient struct {
	baseURL    string
	basicAuth  string
	httpClient *http.Client
}

type CustomerData struct {
	CustomerProperties map[string]CustomerPropertyValue `json:"customer_properties"`
}

type CustomerPropertyValue struct {
	Value json.RawMessage `json:"value"`
}

type CustomerPropertyDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewCDP(basicAuth string) *CDPClient {
	return &CDPClient{
		baseURL:    cdpBaseURL,
		basicAuth:  basicAuth,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (client *CDPClient) GetCustomer(ctx context.Context, customerID string) (CustomerData, error) {
	var customer CustomerData
	if err := client.call(ctx, "/get_customer", map[string]string{"customer_id": customerID}, &customer); err != nil {
		return CustomerData{}, fmt.Errorf("get customer: %w", err)
	}

	return customer, nil
}

func (client *CDPClient) GetCustomerPropertyDefinitions(ctx context.Context) ([]CustomerPropertyDefinition, error) {
	var response struct {
		Properties []CustomerPropertyDefinition `json:"properties"`
	}
	if err := client.call(ctx, "/v1/get_customer_properties_definitions", map[string]any{}, &response); err != nil {
		return nil, fmt.Errorf("get customer property definitions: %w", err)
	}

	return response.Properties, nil
}

func (client *CDPClient) call(ctx context.Context, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Basic "+client.basicAuth)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call API: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return APIError{
			StatusCode: response.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}

	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
