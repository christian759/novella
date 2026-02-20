package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"novella/internal/model"
	"novella/internal/store"
)

type Server struct {
	store *store.Store
}

func New(s *store.Store) *Server {
	return &Server{store: s}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("GET /me", s.requireAuth(s.me))
	mux.HandleFunc("GET /me/bookmarks", s.requireAuth(s.myBookmarks))
	mux.HandleFunc("GET /novels", s.listNovels)
	mux.HandleFunc("POST /novels", s.requireAuth(s.createNovel))
	mux.HandleFunc("/novels/", s.novelSubrouter)
	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const userKey contextKey = "user"

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			respondError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		user, err := s.store.UserByToken(parts[1])
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))
	}
}

func userFromRequest(r *http.Request) (model.User, bool) {
	u, ok := r.Context().Value(userKey).(model.User)
	return u, ok
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, token, err := s.store.Register(req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(w, http.StatusConflict, "email or username already exists")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, token, err := s.store.Login(req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

type createNovelReq struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Genre       string            `json:"genre"`
	Status      model.NovelStatus `json:"status"`
}

func (s *Server) createNovel(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromRequest(r)
	var req createNovelReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	n, err := s.store.CreateNovel(user.ID, req.Title, req.Description, req.Genre, req.Status)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, n)
}

func (s *Server) listNovels(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	includeDrafts := r.URL.Query().Get("include_drafts") == "true"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	authorID, _ := strconv.ParseInt(r.URL.Query().Get("author_id"), 10, 64)

	var requesterID int64
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if parts := strings.SplitN(auth, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		if u, err := s.store.UserByToken(parts[1]); err == nil {
			requesterID = u.ID
		}
	}

	novels := s.store.ListNovels(query, authorID, includeDrafts, requesterID, limit, offset)
	respondJSON(w, http.StatusOK, novels)
}

func (s *Server) novelSubrouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/novels/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	novelID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid novel id")
		return
	}

	if len(parts) == 1 {
		s.handleNovelByID(w, r, novelID)
		return
	}

	switch parts[1] {
	case "chapters":
		s.handleChapters(w, r, novelID, parts[2:])
	case "comments":
		s.handleComments(w, r, novelID)
	case "bookmark":
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			s.handleBookmark(w, r, novelID)
		})(w, r)
	default:
		respondError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleNovelByID(w http.ResponseWriter, r *http.Request, novelID int64) {
	switch r.Method {
	case http.MethodGet:
		var requesterID int64
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if parts := strings.SplitN(auth, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if u, err := s.store.UserByToken(parts[1]); err == nil {
				requesterID = u.ID
			}
		}
		n, err := s.store.NovelByID(novelID, requesterID)
		if err != nil {
			s.handleStoreErr(w, err)
			return
		}
		respondJSON(w, http.StatusOK, n)
	case http.MethodPatch:
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			user, _ := userFromRequest(r)
			var req createNovelReq
			if err := decodeJSON(r, &req); err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			var status *model.NovelStatus
			if req.Status != "" {
				status = &req.Status
			}
			n, err := s.store.UpdateNovel(novelID, user.ID, req.Title, req.Description, req.Genre, status)
			if err != nil {
				s.handleStoreErr(w, err)
				return
			}
			respondJSON(w, http.StatusOK, n)
		})(w, r)
	case http.MethodDelete:
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			user, _ := userFromRequest(r)
			if err := s.store.DeleteNovel(novelID, user.ID); err != nil {
				s.handleStoreErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type chapterReq struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Position int    `json:"position"`
}

func (s *Server) handleChapters(w http.ResponseWriter, r *http.Request, novelID int64, rest []string) {
	if len(rest) == 0 || rest[0] == "" {
		switch r.Method {
		case http.MethodGet:
			var requesterID int64
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			if parts := strings.SplitN(auth, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				if u, err := s.store.UserByToken(parts[1]); err == nil {
					requesterID = u.ID
				}
			}
			chs, err := s.store.ListChapters(novelID, requesterID)
			if err != nil {
				s.handleStoreErr(w, err)
				return
			}
			respondJSON(w, http.StatusOK, chs)
		case http.MethodPost:
			s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
				user, _ := userFromRequest(r)
				var req chapterReq
				if err := decodeJSON(r, &req); err != nil {
					respondError(w, http.StatusBadRequest, err.Error())
					return
				}
				ch, err := s.store.CreateChapter(novelID, user.ID, req.Title, req.Content, req.Position)
				if err != nil {
					s.handleStoreErr(w, err)
					return
				}
				respondJSON(w, http.StatusCreated, ch)
			})(w, r)
		default:
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	chapterID, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid chapter id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		var requesterID int64
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if parts := strings.SplitN(auth, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if u, err := s.store.UserByToken(parts[1]); err == nil {
				requesterID = u.ID
			}
		}
		ch, err := s.store.ChapterByID(novelID, chapterID, requesterID)
		if err != nil {
			s.handleStoreErr(w, err)
			return
		}
		respondJSON(w, http.StatusOK, ch)
	case http.MethodPatch:
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			user, _ := userFromRequest(r)
			var req chapterReq
			if err := decodeJSON(r, &req); err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			ch, err := s.store.UpdateChapter(novelID, chapterID, user.ID, req.Title, req.Content, req.Position)
			if err != nil {
				s.handleStoreErr(w, err)
				return
			}
			respondJSON(w, http.StatusOK, ch)
		})(w, r)
	case http.MethodDelete:
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			user, _ := userFromRequest(r)
			if err := s.store.DeleteChapter(novelID, chapterID, user.ID); err != nil {
				s.handleStoreErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type commentReq struct {
	Body      string `json:"body"`
	ChapterID *int64 `json:"chapter_id"`
}

func (s *Server) handleComments(w http.ResponseWriter, r *http.Request, novelID int64) {
	switch r.Method {
	case http.MethodGet:
		var requesterID int64
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if parts := strings.SplitN(auth, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if u, err := s.store.UserByToken(parts[1]); err == nil {
				requesterID = u.ID
			}
		}
		var chapterID *int64
		if raw := r.URL.Query().Get("chapter_id"); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid chapter_id")
				return
			}
			chapterID = &id
		}
		cs, err := s.store.ListComments(novelID, requesterID, chapterID)
		if err != nil {
			s.handleStoreErr(w, err)
			return
		}
		respondJSON(w, http.StatusOK, cs)
	case http.MethodPost:
		s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			user, _ := userFromRequest(r)
			var req commentReq
			if err := decodeJSON(r, &req); err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			c, err := s.store.CreateComment(novelID, req.ChapterID, user.ID, req.Body)
			if err != nil {
				s.handleStoreErr(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, c)
		})(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type bookmarkReq struct {
	ChapterID *int64 `json:"chapter_id"`
}

func (s *Server) handleBookmark(w http.ResponseWriter, r *http.Request, novelID int64) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, _ := userFromRequest(r)
	var req bookmarkReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := s.store.UpsertBookmark(user.ID, novelID, req.ChapterID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	respondJSON(w, http.StatusOK, b)
}

func (s *Server) myBookmarks(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromRequest(r)
	respondJSON(w, http.StatusOK, s.store.MyBookmarks(user.ID))
}

func (s *Server) handleStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		respondError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, store.ErrUnauthorized):
		respondError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, store.ErrConflict):
		respondError(w, http.StatusConflict, "conflict")
	default:
		respondError(w, http.StatusBadRequest, err.Error())
	}
}

func decodeJSON(r *http.Request, out any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

var _ = context.Background
