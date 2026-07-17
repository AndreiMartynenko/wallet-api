package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AndreiMartynenko/wallet-api/internal/auth"
	"github.com/AndreiMartynenko/wallet-api/internal/user"
)

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register creates a new user with a bcrypt-hashed password.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := s.users.Create(r.Context(), req.Username, hash); err != nil {
		if errors.Is(err, user.ErrExists) {
			writeError(w, http.StatusConflict, "username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	slog.Info("user registered", "username", req.Username)
	w.WriteHeader(http.StatusCreated)
}

// Login verifies credentials and returns a signed JWT.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	u, err := s.users.GetByUsername(r.Context(), req.Username)
	if errors.Is(err, user.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, auth.ErrInvalidCredentials.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		slog.Warn("login failed: bad password", "username", req.Username)
		writeError(w, http.StatusUnauthorized, auth.ErrInvalidCredentials.Error())
		return
	}

	token, err := s.tokens.IssueToken(u.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	slog.Info("user logged in", "username", u.Username)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
