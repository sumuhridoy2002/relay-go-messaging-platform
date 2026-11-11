package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"relay/apps/api/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB *pgxpool.Pool
}

var uuidPattern = regexp.MustCompile(
	`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func getUserID(r *http.Request) (string, bool) {
	return auth.UserIDFromContext(r.Context())
}

// GET /api/v1/conversations
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT
			c.id,
			c.kind,
			c.title,
			c.created_at,
			COALESCE(lm.body, ''),
			COALESCE(lm.message_type, ''),
			lm.created_at,
			other_member.display_name,
			other_member.username,
			other_member.avatar_url,
			(
				SELECT COUNT(*)
				FROM conversation_members cm
				WHERE cm.conversation_id = c.id
			)
		FROM conversations c
		JOIN conversation_members mine
			ON mine.conversation_id = c.id
		LEFT JOIN LATERAL (
			SELECT
				body,
				message_type,
				created_at
			FROM messages
			WHERE conversation_id = c.id
				AND deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT 1
		) lm ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				u.display_name,
				u.username,
				u.avatar_url
			FROM conversation_members cm
			JOIN users u ON u.id = cm.user_id
			WHERE cm.conversation_id = c.id
				AND cm.user_id <> $1
				AND c.kind = 'direct'
			LIMIT 1
		) other_member ON TRUE
		WHERE mine.user_id = $1
		ORDER BY COALESCE(lm.created_at, c.created_at) DESC
	`, userID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to fetch conversations",
		})
		return
	}
	defer rows.Close()

	type Conversation struct {
		ID                  string  `json:"id"`
		Kind                string  `json:"kind"`
		Title               *string `json:"title"`
		CreatedAt           any     `json:"created_at"`
		LastMessage         string  `json:"last_message"`
		MessageType         string  `json:"last_message_type"`
		LastMessageAt       any     `json:"last_message_at"`
		OtherMemberName     *string `json:"other_member_name"`
		OtherMemberUsername *string `json:"other_member_username"`
		OtherMemberAvatar   *string `json:"other_member_avatar"`
		MemberCount         int     `json:"member_count"`
	}

	conversations := []Conversation{}

	for rows.Next() {
		var c Conversation

		if err := rows.Scan(
			&c.ID,
			&c.Kind,
			&c.Title,
			&c.CreatedAt,
			&c.LastMessage,
			&c.MessageType,
			&c.LastMessageAt,
			&c.OtherMemberName,
			&c.OtherMemberUsername,
			&c.OtherMemberAvatar,
			&c.MemberCount,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Unable to read conversations",
			})
			return
		}

		conversations = append(conversations, c)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to read conversations",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversations": conversations,
	})
}

// POST /api/v1/conversations
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	var input struct {
		Kind      string   `json:"kind"`
		Title     string   `json:"title"`
		MemberIDs []string `json:"member_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	input.Kind = strings.TrimSpace(input.Kind)
	input.Title = strings.TrimSpace(input.Title)

	if input.Kind != "direct" && input.Kind != "group" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Kind must be direct or group",
		})
		return
	}

	if input.Kind == "direct" && len(input.MemberIDs) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Direct chat requires exactly one other member",
		})
		return
	}

	if input.Kind == "group" && len(input.MemberIDs) < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Group chat requires at least one other member",
		})
		return
	}

	if input.Kind == "group" && input.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Group title is required",
		})
		return
	}

	seen := map[string]bool{userID: true}

	for _, id := range input.MemberIDs {
		if !uuidPattern.MatchString(id) || seen[id] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Invalid or duplicate member ID",
			})
			return
		}

		seen[id] = true
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to start transaction",
		})
		return
	}
	defer func() {
		_ = tx.Rollback(r.Context())
	}()

	// Serialize creation for a direct pair. The lock key is symmetric, so
	// requests A->B and B->A acquire the same transaction-scoped lock.
	if input.Kind == "direct" {
		otherID := input.MemberIDs[0]
		_, err = tx.Exec(r.Context(), `
			SELECT pg_advisory_xact_lock(
				hashtextextended(
					LEAST($1::text, $2::text) || ':' || GREATEST($1::text, $2::text),
					0
				)
			)
		`, userID, otherID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Unable to lock direct conversation",
			})
			return
		}

		var existingID string
		err = tx.QueryRow(r.Context(), `
			SELECT COALESCE((
				SELECT c.id::text
				FROM conversations c
				JOIN conversation_members mine
					ON mine.conversation_id = c.id AND mine.user_id = $1
				JOIN conversation_members other
					ON other.conversation_id = c.id AND other.user_id = $2
				WHERE c.kind = 'direct'
				  AND (
					SELECT COUNT(*)
					FROM conversation_members cm
					WHERE cm.conversation_id = c.id
				  ) = 2
				LIMIT 1
			), '')
		`, userID, otherID).Scan(&existingID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Unable to check existing conversation",
			})
			return
		}

		if existingID != "" {
			if err := tx.Commit(r.Context()); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "Unable to finish conversation lookup",
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"message": "Existing conversation",
				"conversation": map[string]string{
					"id":   existingID,
					"kind": "direct",
				},
			})
			return
		}
	}

	var conversationID string

	err = tx.QueryRow(r.Context(), `
		INSERT INTO conversations (kind, title, created_by)
		VALUES ($1, NULLIF($2, ''), $3)
		RETURNING id
	`, input.Kind, input.Title, userID).Scan(&conversationID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to create conversation",
		})
		return
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, conversationID, userID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to add conversation owner",
		})
		return
	}

	for _, memberID := range input.MemberIDs {
		_, err = tx.Exec(r.Context(), `
			INSERT INTO conversation_members (conversation_id, user_id, role)
			VALUES ($1, $2, 'member')
		`, conversationID, memberID)

		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "One or more members do not exist",
			})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to save conversation",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Conversation created",
		"conversation": map[string]string{
			"id":    conversationID,
			"kind":  input.Kind,
			"title": input.Title,
		},
	})
}

// GET /api/v1/conversations/{id}/messages
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	conversationID := chi.URLParam(r, "id")

	if !uuidPattern.MatchString(conversationID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid conversation ID",
		})
		return
	}

	var exists bool

	err := h.DB.QueryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM conversation_members
			WHERE conversation_id = $1
				AND user_id = $2
		)
	`, conversationID, userID).Scan(&exists)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to verify membership",
		})
		return
	}

	if !exists {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "You are not a member of this conversation",
		})
		return
	}

	limit := 50

	if raw := r.URL.Query().Get("limit"); raw != "" {
		var n int

		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Limit must be between 1 and 100",
			})
			return
		}

		limit = n
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT * FROM (
			SELECT
				m.id,
				m.conversation_id,
				m.sender_id,
				u.display_name,
				u.username,
				u.avatar_url,
				m.body,
				m.message_type,
				m.created_at,
				m.edited_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
				AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC
			LIMIT $2
		) recent
		ORDER BY created_at ASC
	`, conversationID, limit)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to fetch messages",
		})
		return
	}
	defer rows.Close()

	type Message struct {
		ID             string  `json:"id"`
		ConversationID string  `json:"conversation_id"`
		SenderID       string  `json:"sender_id"`
		DisplayName    string  `json:"display_name"`
		Username       string  `json:"username"`
		AvatarURL      *string `json:"avatar_url"`
		Body           *string `json:"body"`
		MessageType    string  `json:"message_type"`
		CreatedAt      any     `json:"created_at"`
		EditedAt       any     `json:"edited_at"`
	}

	messages := []Message{}

	for rows.Next() {
		var m Message

		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.DisplayName,
			&m.Username,
			&m.AvatarURL,
			&m.Body,
			&m.MessageType,
			&m.CreatedAt,
			&m.EditedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Unable to read messages",
			})
			return
		}

		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to read messages",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
	})
}

// POST /api/v1/conversations/{id}/messages
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	conversationID := chi.URLParam(r, "id")

	if !uuidPattern.MatchString(conversationID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid conversation ID",
		})
		return
	}

	var input struct {
		Body        string `json:"body"`
		MessageType string `json:"message_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	input.Body = strings.TrimSpace(input.Body)

	if input.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Message body is required",
		})
		return
	}

	if len(input.Body) > 10000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Message is too long",
		})
		return
	}

	if input.MessageType == "" {
		input.MessageType = "text"
	}

	if input.MessageType != "text" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Only text messages are currently supported",
		})
		return
	}

	var exists bool

	err := h.DB.QueryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM conversation_members
			WHERE conversation_id = $1
				AND user_id = $2
		)
	`, conversationID, userID).Scan(&exists)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to verify membership",
		})
		return
	}

	if !exists {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "You are not a member of this conversation",
		})
		return
	}

	var message struct {
		ID             string `json:"id"`
		ConversationID string `json:"conversation_id"`
		SenderID       string `json:"sender_id"`
		Body           string `json:"body"`
		MessageType    string `json:"message_type"`
		CreatedAt      any    `json:"created_at"`
	}

	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO messages (
			conversation_id,
			sender_id,
			body,
			message_type
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			conversation_id,
			sender_id,
			body,
			message_type,
			created_at
	`, conversationID, userID, input.Body, input.MessageType).Scan(
		&message.ID,
		&message.ConversationID,
		&message.SenderID,
		&message.Body,
		&message.MessageType,
		&message.CreatedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to send message",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Message sent",
		"data":    message,
	})
}

// GET /api/v1/users/search?q=sakib
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))

	if len(query) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Search query must be at least 2 characters",
		})
		return
	}

	if len(query) > 50 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Search query is too long",
		})
		return
	}

	rows, err := h.DB.Query(r.Context(), `
        SELECT
            id,
            username,
            display_name,
            avatar_url
        FROM users
        WHERE id <> $1
            AND (
                username ILIKE $2
                OR display_name ILIKE $2
            )
        ORDER BY
            CASE
                WHEN username ILIKE $3 THEN 0
                ELSE 1
            END,
            display_name ASC
        LIMIT 20
    `, userID, "%"+query+"%", query+"%")

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to search users",
		})
		return
	}
	defer rows.Close()

	type User struct {
		ID          string  `json:"id"`
		Username    string  `json:"username"`
		DisplayName string  `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
	}

	users := []User{}

	for rows.Next() {
		var user User

		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.AvatarURL,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Unable to read users",
			})
			return
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to read users",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
	})
}
