package auth

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	DB        *pgxpool.Pool
	JWTSecret []byte
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,24}$`)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) generateToken(userID string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.JWTSecret)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)

	parsedEmail, emailErr := mail.ParseAddress(input.Email)
	if emailErr != nil || parsedEmail.Address != input.Email || len(input.Email) > 254 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "A valid email address is required",
		})
		return
	}
	if !usernamePattern.MatchString(input.Username) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Username must be 3-24 characters and contain only letters, numbers, or underscores",
		})
		return
	}
	if input.DisplayName == "" || utf8.RuneCountInString(input.DisplayName) > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Display name must be between 1 and 60 characters",
		})
		return
	}
	if len(input.Password) < 8 || len(input.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Password must be between 8 and 72 bytes",
		})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not process password",
		})
		return
	}

	var user User

	err = h.DB.QueryRow(
		r.Context(),
		`INSERT INTO users (email, username, display_name, password_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, username, display_name, avatar_url, created_at`,
		input.Email,
		input.Username,
		input.DisplayName,
		string(passwordHash),
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.CreatedAt,
	)

	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "Email or username already exists",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to create account",
		})
		return
	}

	accessToken, err := h.generateToken(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not generate access token",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"access_token": accessToken,
		"user":         user,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input LoginRequest

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if input.Email == "" || len(input.Email) > 254 || input.Password == "" || len(input.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email and password are required",
		})
		return
	}

	var user User
	var passwordHash string

	err := h.DB.QueryRow(
		r.Context(),
		`SELECT id, email, username, display_name, avatar_url,
		        created_at, password_hash
		 FROM users
		 WHERE email = $1`,
		input.Email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.CreatedAt,
		&passwordHash,
	)

	if err != nil || bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(input.Password),
	) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid email or password",
		})
		return
	}

	accessToken, err := h.generateToken(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not generate access token",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"user":         user,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
		return
	}

	var user User

	err := h.DB.QueryRow(
		r.Context(),
		`SELECT id, email, username, display_name, avatar_url, created_at
		 FROM users
		 WHERE id = $1`,
		userID,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.CreatedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}
