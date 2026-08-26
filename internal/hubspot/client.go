package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.hubapi.com"

var errContactNotFound = errors.New("HubSpot contact not found")

type Client struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
}

func (error APIError) Error() string {
	return fmt.Sprintf("HubSpot API returned HTTP %d: %s", error.StatusCode, error.Body)
}

type ContactInput struct {
	Email     string
	FirstName string
	LastName  string
}

type contactResponse struct {
	ID string `json:"id"`
}

func New(accessToken string) *Client {
	return &Client{
		baseURL:     defaultBaseURL,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (client *Client) UpsertContact(ctx context.Context, input ContactInput) (string, error) {
	contactID, err := client.findContactByEmail(ctx, input.Email)
	if err == nil {
		if err := client.updateContact(ctx, contactID, input); err != nil {
			return "", err
		}
		return contactID, nil
	}
	if !errors.Is(err, errContactNotFound) {
		return "", err
	}

	return client.createContact(ctx, input)
}

func (client *Client) CreateNote(ctx context.Context, contactID, body string, timestamp time.Time) error {
	payload := map[string]any{
		"properties": map[string]string{
			"hs_timestamp": timestamp.UTC().Format(time.RFC3339),
			"hs_note_body": body,
		},
		"associations": []any{
			map[string]any{
				"to": map[string]string{"id": contactID},
				"types": []any{
					map[string]any{
						"associationCategory": "HUBSPOT_DEFINED",
						"associationTypeId":   202,
					},
				},
			},
		},
	}

	return client.call(ctx, http.MethodPost, "/crm/objects/2026-03/notes", payload, nil)
}

func (client *Client) findContactByEmail(ctx context.Context, email string) (string, error) {
	path := "/crm/objects/2026-03/contacts/" + url.PathEscape(email) + "?idProperty=email&properties=email,firstname,lastname"
	var response contactResponse
	if err := client.call(ctx, http.MethodGet, path, nil, &response); err != nil {
		var apiError APIError
		if errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound {
			return "", errContactNotFound
		}
		return "", fmt.Errorf("find contact by email: %w", err)
	}

	return response.ID, nil
}

func (client *Client) createContact(ctx context.Context, input ContactInput) (string, error) {
	payload := map[string]map[string]string{
		"properties": contactProperties(input, true),
	}
	var response contactResponse
	if err := client.call(ctx, http.MethodPost, "/crm/objects/2026-03/contacts", payload, &response); err != nil {
		return "", fmt.Errorf("create contact: %w", err)
	}

	return response.ID, nil
}

func (client *Client) updateContact(ctx context.Context, contactID string, input ContactInput) error {
	properties := contactProperties(input, false)
	if len(properties) == 0 {
		return nil
	}

	payload := map[string]map[string]string{"properties": properties}
	if err := client.call(ctx, http.MethodPatch, "/crm/objects/2026-03/contacts/"+url.PathEscape(contactID), payload, nil); err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	return nil
}

func contactProperties(input ContactInput, includeEmail bool) map[string]string {
	properties := make(map[string]string, 3)
	if includeEmail {
		properties["email"] = input.Email
	}
	if input.FirstName != "" {
		properties["firstname"] = input.FirstName
	}
	if input.LastName != "" {
		properties["lastname"] = input.LastName
	}

	return properties
}

func (client *Client) call(ctx context.Context, method, path string, payload any, target any) error {
	var requestBody io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		requestBody = bytes.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.accessToken)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call API: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return APIError{
			StatusCode: response.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}

	if target != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
