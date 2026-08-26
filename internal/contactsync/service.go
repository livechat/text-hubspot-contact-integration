package contactsync

import (
	"cmp"
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/livechat/text-hubspot-contact-sync/internal/hubspot"
	"github.com/livechat/text-hubspot-contact-sync/internal/textapi"
)

const (
	defaultSummaryPollInterval       = 2 * time.Second
	defaultSummaryPollRequestTimeout = 10 * time.Second
	defaultSummaryPollTimeout        = 60 * time.Second
)

type textClient interface {
	GetChat(context.Context, string, string) (textapi.Chat, error)
	GetCustomer(context.Context, string) (textapi.Customer, error)
	RequestThreadSummary(context.Context, string, string) error
}

type hubSpotClient interface {
	UpsertContact(context.Context, hubspot.ContactInput) (string, error)
	CreateNote(context.Context, string, string, time.Time) error
}

type Service struct {
	textClient         textClient
	hubSpotClient      hubSpotClient
	now                func() time.Time
	pollInterval       time.Duration
	pollRequestTimeout time.Duration
	pollTimeout        time.Duration
}

type Result struct {
	ContactID   string
	NoteCreated bool
	SkipReason  string
}

func New(textClient textClient, hubSpotClient hubSpotClient) *Service {
	return &Service{
		textClient:         textClient,
		hubSpotClient:      hubSpotClient,
		now:                time.Now,
		pollInterval:       defaultSummaryPollInterval,
		pollRequestTimeout: defaultSummaryPollRequestTimeout,
		pollTimeout:        defaultSummaryPollTimeout,
	}
}

func (service *Service) SyncThread(ctx context.Context, chatID, threadID string) (Result, error) {
	chat, err := service.textClient.GetChat(ctx, chatID, threadID)
	if err != nil {
		return Result{}, fmt.Errorf("get Text chat: %w", err)
	}
	customerID := customerID(chat.Users)
	if customerID == "" {
		return Result{SkipReason: "chat has no customer"}, nil
	}

	customer, err := service.textClient.GetCustomer(ctx, customerID)
	if err != nil {
		return Result{}, fmt.Errorf("get Text customer: %w", err)
	}

	contact, ok := contactInput(customer)
	if !ok {
		return Result{SkipReason: "customer has no email"}, nil
	}

	chat, err = service.waitForSummary(ctx, chatID, threadID, chat)
	if err != nil {
		return Result{}, err
	}

	contactID, err := service.hubSpotClient.UpsertContact(ctx, contact)
	if err != nil {
		return Result{}, fmt.Errorf("upsert HubSpot contact: %w", err)
	}

	timestamp := chat.Thread.Summary.UpdatedAt
	if timestamp.IsZero() {
		timestamp = service.now()
	}

	if err := service.hubSpotClient.CreateNote(
		ctx,
		contactID,
		noteBody(chat.Thread.Summary, chat.ID, chat.Thread.ID),
		timestamp,
	); err != nil {
		return Result{}, fmt.Errorf("create HubSpot note: %w", err)
	}

	return Result{ContactID: contactID, NoteCreated: true}, nil
}

func (service *Service) waitForSummary(ctx context.Context, chatID, threadID string, chat textapi.Chat) (textapi.Chat, error) {
	if hasSummary(chat) {
		return chat, nil
	}

	if err := service.textClient.RequestThreadSummary(ctx, chatID, threadID); err != nil {
		return textapi.Chat{}, fmt.Errorf("request Text thread summary: %w", err)
	}

	pollContext, cancel := context.WithTimeout(ctx, service.pollTimeout)
	defer cancel()
	ticker := time.NewTicker(service.pollInterval)
	defer ticker.Stop()
	var lastPollError error

	for {
		select {
		case <-pollContext.Done():
			if lastPollError != nil {
				return textapi.Chat{}, fmt.Errorf("wait for Text thread summary after poll failure: %w", lastPollError)
			}
			return textapi.Chat{}, fmt.Errorf("wait for Text thread summary: %w", pollContext.Err())
		case <-ticker.C:
			pollRequestContext, cancel := context.WithTimeout(pollContext, service.pollRequestTimeout)
			updatedChat, err := service.textClient.GetChat(pollRequestContext, chatID, threadID)
			cancel()
			if err != nil {
				lastPollError = err
				continue
			}
			if hasSummary(updatedChat) {
				return updatedChat, nil
			}
		}
	}
}

func hasSummary(chat textapi.Chat) bool {
	if chat.Thread.Summary == nil || chat.Thread.Summary.Status != "ok" {
		return false
	}

	if strings.TrimSpace(chat.Thread.Summary.Content.Title) != "" {
		return true
	}
	for _, bullet := range chat.Thread.Summary.Content.SummaryBullets {
		if strings.TrimSpace(bullet) != "" {
			return true
		}
	}

	return false
}

func customerID(users []textapi.User) string {
	for _, user := range users {
		if user.Type == "customer" {
			return user.ID
		}
	}

	return ""
}

func contactInput(customer textapi.Customer) (hubspot.ContactInput, bool) {
	email := strings.TrimSpace(customer.Email)
	if email == "" {
		return hubspot.ContactInput{}, false
	}

	firstName, lastName := splitName(customer.Name)
	return hubspot.ContactInput{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, true
}

func splitName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], strings.Join(parts[1:], " ")
}

func noteBody(summary *textapi.Summary, chatID, threadID string) string {
	title := cmp.Or(strings.TrimSpace(summary.Content.Title), "Chat summary")

	bullets := make([]string, 0, len(summary.Content.SummaryBullets))
	for _, bullet := range summary.Content.SummaryBullets {
		if bullet = strings.TrimSpace(bullet); bullet != "" {
			bullets = append(bullets, "<li>"+html.EscapeString(bullet)+"</li>")
		}
	}

	body := "<p><strong>" + html.EscapeString(title) + "</strong></p>"
	if len(bullets) > 0 {
		body += "<ul>" + strings.Join(bullets, "") + "</ul>"
	}

	return fmt.Sprintf(
		"%s<p>Chat ID: %s<br>Thread ID: %s</p>",
		body,
		html.EscapeString(chatID),
		html.EscapeString(threadID),
	)
}
