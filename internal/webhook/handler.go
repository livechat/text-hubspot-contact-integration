package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/livechat/text-hubspot-contact-sync/internal/contactsync"
)

const (
	syncTimeout = 2 * time.Minute
)

type synchronizer interface {
	SyncThread(context.Context, string, string) (contactsync.Result, error)
}

type Handler struct {
	secret       string
	synchronizer synchronizer
	logger       *slog.Logger
}

type notification struct {
	Action    string  `json:"action"`
	SecretKey string  `json:"secret_key"`
	Payload   payload `json:"payload"`
}

type payload struct {
	ChatID   string `json:"chat_id"`
	ThreadID string `json:"thread_id"`
}

func NewHandler(secret string, synchronizer synchronizer, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		secret:       secret,
		synchronizer: synchronizer,
		logger:       logger,
	}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var notification notification
	if err := json.NewDecoder(request.Body).Decode(&notification); err != nil {
		http.Error(writer, "invalid webhook payload", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare([]byte(notification.SecretKey), []byte(handler.secret)) != 1 {
		http.Error(writer, "invalid webhook secret", http.StatusUnauthorized)
		return
	}

	if notification.Action != "chat_deactivated" {
		handler.logger.Info("ignoring unsupported Text webhook", "action", notification.Action)
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	var (
		chatID   = strings.TrimSpace(notification.Payload.ChatID)
		threadID = strings.TrimSpace(notification.Payload.ThreadID)
	)

	if chatID == "" || threadID == "" {
		http.Error(writer, "chat_id and thread_id are required", http.StatusBadRequest)
		return
	}

	// Text can close its webhook connection while the asynchronous summary is generated.
	go handler.syncThread(chatID, threadID)
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) syncThread(chatID, threadID string) {
	syncContext, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	result, err := handler.synchronizer.SyncThread(syncContext, chatID, threadID)
	if err != nil {
		handler.logger.Error("sync deactivated Text chat", "chat_id", chatID, "thread_id", threadID, "error", err)
		return
	}

	if result.SkipReason != "" {
		handler.logger.Info("skipped Text chat sync", "chat_id", chatID, "thread_id", threadID, "reason", result.SkipReason)
		return
	}

	handler.logger.Info("synced Text chat", "chat_id", chatID, "thread_id", threadID, "contact_id", result.ContactID, "note_created", result.NoteCreated)
}
