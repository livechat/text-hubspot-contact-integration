package textapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.livechatinc.com/v3.6"

type Client struct {
	baseURL    string
	basicAuth  string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
}

func (error APIError) Error() string {
	return fmt.Sprintf("Text API returned HTTP %d: %s", error.StatusCode, error.Body)
}

type Chat struct {
	ID     string `json:"id"`
	Thread Thread `json:"thread"`
	Users  []User `json:"users"`
}

type Thread struct {
	ID      string   `json:"id"`
	Summary *Summary `json:"summary"`
}

type Summary struct {
	Status    string         `json:"status"`
	Content   SummaryContent `json:"content"`
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type SummaryContent struct {
	Title          string   `json:"title"`
	SummaryBullets []string `json:"summary_bullets"`
}

type User struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func New(basicAuth string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		basicAuth:  basicAuth,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (client *Client) GetChat(ctx context.Context, chatID, threadID string) (Chat, error) {
	var chat Chat
	if err := client.call(ctx, "get_chat", map[string]string{
		"chat_id":   chatID,
		"thread_id": threadID,
	}, &chat); err != nil {
		return Chat{}, fmt.Errorf("get chat: %w", err)
	}

	return chat, nil
}

func (client *Client) GetCustomer(ctx context.Context, customerID string) (Customer, error) {
	var customer Customer
	if err := client.call(ctx, "get_customer", map[string]string{"id": customerID}, &customer); err != nil {
		return Customer{}, fmt.Errorf("get customer: %w", err)
	}

	return customer, nil
}

func (client *Client) RequestThreadSummary(ctx context.Context, chatID, threadID string) error {
	if err := client.call(ctx, "request_thread_summary", map[string]string{
		"chat_id":   chatID,
		"thread_id": threadID,
	}, nil); err != nil {
		if isSummaryAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("request thread summary: %w", err)
	}

	return nil
}

func isSummaryAlreadyExists(err error) bool {
	var apiError APIError
	if errors.As(err, &apiError) && apiError.StatusCode == http.StatusConflict {
		return true
	}

	return false
}

func (client *Client) call(ctx context.Context, action string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.baseURL+"/agent/action/"+action,
		bytes.NewReader(body),
	)
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

	if target != nil {
		if err := json.Unmarshal(responseBody, target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
