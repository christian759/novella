package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"novella/internal/model"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
)

type Store struct {
	mu     sync.RWMutex
	dbPath string

	usersByID       map[int64]model.User
	usersByEmail    map[string]int64
	usersByUsername map[string]int64

	novelsByID map[int64]model.Novel

	chaptersByID      map[int64]model.Chapter
	chapterIDsByNovel map[int64][]int64

	commentsByID      map[int64]model.Comment
	commentIDsByNovel map[int64][]int64

	bookmarks map[string]model.Bookmark
	sessions  map[string]int64

	nextUserID    int64
	nextNovelID   int64
	nextChapterID int64
	nextCommentID int64
}

func New() *Store {
	s, _ := NewWithDB("")
	return s
}

func NewWithDB(dbPath string) (*Store, error) {
	s := &Store{
		dbPath:            strings.TrimSpace(dbPath),
		usersByID:         make(map[int64]model.User),
		usersByEmail:      make(map[string]int64),
		usersByUsername:   make(map[string]int64),
		novelsByID:        make(map[int64]model.Novel),
		chaptersByID:      make(map[int64]model.Chapter),
		chapterIDsByNovel: make(map[int64][]int64),
		commentsByID:      make(map[int64]model.Comment),
		commentIDsByNovel: make(map[int64][]int64),
		bookmarks:         make(map[string]model.Bookmark),
		sessions:          make(map[string]int64),
	}
	if s.dbPath == "" {
		return s, nil
	}
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func normalize(input string) string {
	return strings.TrimSpace(strings.ToLower(input))
}

func hashPassword(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) Register(username, email, password string) (model.User, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := normalize(username)
	e := normalize(email)
	if u == "" || e == "" || password == "" {
		return model.User{}, "", fmt.Errorf("username, email, and password are required")
	}
	if _, exists := s.usersByUsername[u]; exists {
		return model.User{}, "", ErrConflict
	}
	if _, exists := s.usersByEmail[e]; exists {
		return model.User{}, "", ErrConflict
	}

	s.nextUserID++
	salt, err := randomHex(16)
	if err != nil {
		return model.User{}, "", err
	}
	user := model.User{
		ID:           s.nextUserID,
		Username:     strings.TrimSpace(username),
		Email:        strings.TrimSpace(email),
		PasswordSalt: salt,
		PasswordHash: hashPassword(salt, password),
		CreatedAt:    time.Now().UTC(),
	}
	s.usersByID[user.ID] = user
	s.usersByEmail[e] = user.ID
	s.usersByUsername[u] = user.ID

	token, err := randomHex(32)
	if err != nil {
		return model.User{}, "", err
	}
	s.sessions[token] = user.ID
	if err := s.persistLocked(); err != nil {
		return model.User{}, "", err
	}
	return user, token, nil
}

func (s *Store) Login(email, password string) (model.User, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uid, ok := s.usersByEmail[normalize(email)]
	if !ok {
		return model.User{}, "", ErrUnauthorized
	}
	user := s.usersByID[uid]
	if user.PasswordHash != hashPassword(user.PasswordSalt, password) {
		return model.User{}, "", ErrUnauthorized
	}
	token, err := randomHex(32)
	if err != nil {
		return model.User{}, "", err
	}
	s.sessions[token] = user.ID
	if err := s.persistLocked(); err != nil {
		return model.User{}, "", err
	}
	return user, token, nil
}

func (s *Store) UserByToken(token string) (model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uid, ok := s.sessions[token]
	if !ok {
		return model.User{}, ErrUnauthorized
	}
	user, ok := s.usersByID[uid]
	if !ok {
		return model.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *Store) CreateNovel(authorID int64, title, description, genre string, status model.NovelStatus) (model.Novel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if status == "" {
		status = model.NovelDraft
	}
	if status != model.NovelDraft && status != model.NovelPublished {
		return model.Novel{}, fmt.Errorf("invalid status")
	}
	if strings.TrimSpace(title) == "" {
		return model.Novel{}, fmt.Errorf("title is required")
	}
	s.nextNovelID++
	now := time.Now().UTC()
	n := model.Novel{
		ID:          s.nextNovelID,
		AuthorID:    authorID,
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Genre:       strings.TrimSpace(genre),
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.novelsByID[n.ID] = n
	if err := s.persistLocked(); err != nil {
		return model.Novel{}, err
	}
	return n, nil
}

func (s *Store) ListNovels(query string, authorID int64, includeDrafts bool, requesterID int64, limit, offset int) []model.Novel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := normalize(query)
	result := make([]model.Novel, 0, len(s.novelsByID))
	for _, n := range s.novelsByID {
		if authorID > 0 && n.AuthorID != authorID {
			continue
		}
		if !includeDrafts && n.Status != model.NovelPublished && n.AuthorID != requesterID {
			continue
		}
		if q != "" {
			blob := normalize(n.Title + " " + n.Description + " " + n.Genre)
			if !strings.Contains(blob, q) {
				continue
			}
		}
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })

	if offset > len(result) {
		return []model.Novel{}
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

func (s *Store) NovelByID(id int64, requesterID int64) (model.Novel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.novelsByID[id]
	if !ok {
		return model.Novel{}, ErrNotFound
	}
	if n.Status != model.NovelPublished && n.AuthorID != requesterID {
		return model.Novel{}, ErrUnauthorized
	}
	return n, nil
}

func (s *Store) UpdateNovel(id, requesterID int64, title, description, genre string, status *model.NovelStatus) (model.Novel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[id]
	if !ok {
		return model.Novel{}, ErrNotFound
	}
	if n.AuthorID != requesterID {
		return model.Novel{}, ErrUnauthorized
	}
	if strings.TrimSpace(title) != "" {
		n.Title = strings.TrimSpace(title)
	}
	if description != "" {
		n.Description = strings.TrimSpace(description)
	}
	if genre != "" {
		n.Genre = strings.TrimSpace(genre)
	}
	if status != nil {
		if *status != model.NovelDraft && *status != model.NovelPublished {
			return model.Novel{}, fmt.Errorf("invalid status")
		}
		n.Status = *status
	}
	n.UpdatedAt = time.Now().UTC()
	s.novelsByID[id] = n
	if err := s.persistLocked(); err != nil {
		return model.Novel{}, err
	}
	return n, nil
}

func (s *Store) DeleteNovel(id, requesterID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[id]
	if !ok {
		return ErrNotFound
	}
	if n.AuthorID != requesterID {
		return ErrUnauthorized
	}
	delete(s.novelsByID, id)
	for _, cid := range s.chapterIDsByNovel[id] {
		delete(s.chaptersByID, cid)
	}
	delete(s.chapterIDsByNovel, id)
	for _, cmid := range s.commentIDsByNovel[id] {
		delete(s.commentsByID, cmid)
	}
	delete(s.commentIDsByNovel, id)
	for k, b := range s.bookmarks {
		if b.NovelID == id {
			delete(s.bookmarks, k)
		}
	}
	if err := s.persistLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Store) CreateChapter(novelID, requesterID int64, title, content string, position int) (model.Chapter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return model.Chapter{}, ErrNotFound
	}
	if n.AuthorID != requesterID {
		return model.Chapter{}, ErrUnauthorized
	}
	if strings.TrimSpace(title) == "" {
		return model.Chapter{}, fmt.Errorf("title is required")
	}
	s.nextChapterID++
	now := time.Now().UTC()
	if position <= 0 {
		position = len(s.chapterIDsByNovel[novelID]) + 1
	}
	ch := model.Chapter{
		ID:        s.nextChapterID,
		NovelID:   novelID,
		Title:     strings.TrimSpace(title),
		Content:   content,
		Position:  position,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.chaptersByID[ch.ID] = ch
	s.chapterIDsByNovel[novelID] = append(s.chapterIDsByNovel[novelID], ch.ID)
	n.UpdatedAt = now
	s.novelsByID[novelID] = n
	if err := s.persistLocked(); err != nil {
		return model.Chapter{}, err
	}
	return ch, nil
}

func (s *Store) ListChapters(novelID, requesterID int64) ([]model.Chapter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return nil, ErrNotFound
	}
	if n.Status != model.NovelPublished && n.AuthorID != requesterID {
		return nil, ErrUnauthorized
	}
	res := make([]model.Chapter, 0, len(s.chapterIDsByNovel[novelID]))
	for _, id := range s.chapterIDsByNovel[novelID] {
		res = append(res, s.chaptersByID[id])
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Position < res[j].Position })
	return res, nil
}

func (s *Store) ChapterByID(novelID, chapterID, requesterID int64) (model.Chapter, error) {
	chapters, err := s.ListChapters(novelID, requesterID)
	if err != nil {
		return model.Chapter{}, err
	}
	for _, ch := range chapters {
		if ch.ID == chapterID {
			return ch, nil
		}
	}
	return model.Chapter{}, ErrNotFound
}

func (s *Store) UpdateChapter(novelID, chapterID, requesterID int64, title, content string, position int) (model.Chapter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return model.Chapter{}, ErrNotFound
	}
	if n.AuthorID != requesterID {
		return model.Chapter{}, ErrUnauthorized
	}
	ch, ok := s.chaptersByID[chapterID]
	if !ok || ch.NovelID != novelID {
		return model.Chapter{}, ErrNotFound
	}
	if strings.TrimSpace(title) != "" {
		ch.Title = strings.TrimSpace(title)
	}
	if content != "" {
		ch.Content = content
	}
	if position > 0 {
		ch.Position = position
	}
	ch.UpdatedAt = time.Now().UTC()
	s.chaptersByID[chapterID] = ch
	n.UpdatedAt = ch.UpdatedAt
	s.novelsByID[novelID] = n
	if err := s.persistLocked(); err != nil {
		return model.Chapter{}, err
	}
	return ch, nil
}

func (s *Store) DeleteChapter(novelID, chapterID, requesterID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return ErrNotFound
	}
	if n.AuthorID != requesterID {
		return ErrUnauthorized
	}
	ch, ok := s.chaptersByID[chapterID]
	if !ok || ch.NovelID != novelID {
		return ErrNotFound
	}
	delete(s.chaptersByID, chapterID)
	ids := s.chapterIDsByNovel[novelID]
	for i := range ids {
		if ids[i] == chapterID {
			s.chapterIDsByNovel[novelID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	for k, b := range s.bookmarks {
		if b.NovelID == novelID && b.ChapterID != nil && *b.ChapterID == chapterID {
			b.ChapterID = nil
			b.ChapterPos = nil
			s.bookmarks[k] = b
		}
	}
	if err := s.persistLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Store) CreateComment(novelID int64, chapterID *int64, userID int64, body string) (model.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(body) == "" {
		return model.Comment{}, fmt.Errorf("body is required")
	}
	n, ok := s.novelsByID[novelID]
	if !ok {
		return model.Comment{}, ErrNotFound
	}
	if n.Status != model.NovelPublished && n.AuthorID != userID {
		return model.Comment{}, ErrUnauthorized
	}
	if chapterID != nil {
		ch, ok := s.chaptersByID[*chapterID]
		if !ok || ch.NovelID != novelID {
			return model.Comment{}, ErrNotFound
		}
	}
	s.nextCommentID++
	cm := model.Comment{
		ID:        s.nextCommentID,
		NovelID:   novelID,
		ChapterID: chapterID,
		UserID:    userID,
		Body:      strings.TrimSpace(body),
		CreatedAt: time.Now().UTC(),
	}
	s.commentsByID[cm.ID] = cm
	s.commentIDsByNovel[novelID] = append(s.commentIDsByNovel[novelID], cm.ID)
	if err := s.persistLocked(); err != nil {
		return model.Comment{}, err
	}
	return cm, nil
}

func (s *Store) ListComments(novelID, requesterID int64, chapterID *int64) ([]model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return nil, ErrNotFound
	}
	if n.Status != model.NovelPublished && n.AuthorID != requesterID {
		return nil, ErrUnauthorized
	}
	res := make([]model.Comment, 0, len(s.commentIDsByNovel[novelID]))
	for _, id := range s.commentIDsByNovel[novelID] {
		c := s.commentsByID[id]
		if chapterID != nil {
			if c.ChapterID == nil || *c.ChapterID != *chapterID {
				continue
			}
		}
		res = append(res, c)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func bookmarkKey(userID, novelID int64) string {
	return fmt.Sprintf("%d:%d", userID, novelID)
}

func (s *Store) UpsertBookmark(userID, novelID int64, chapterID *int64) (model.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.novelsByID[novelID]
	if !ok {
		return model.Bookmark{}, ErrNotFound
	}
	if n.Status != model.NovelPublished && n.AuthorID != userID {
		return model.Bookmark{}, ErrUnauthorized
	}
	var pos *int
	if chapterID != nil {
		ch, ok := s.chaptersByID[*chapterID]
		if !ok || ch.NovelID != novelID {
			return model.Bookmark{}, ErrNotFound
		}
		cp := ch.Position
		pos = &cp
	}
	b := model.Bookmark{
		UserID:     userID,
		NovelID:    novelID,
		ChapterID:  chapterID,
		UpdatedAt:  time.Now().UTC(),
		ChapterPos: pos,
	}
	s.bookmarks[bookmarkKey(userID, novelID)] = b
	if err := s.persistLocked(); err != nil {
		return model.Bookmark{}, err
	}
	return b, nil
}

func (s *Store) MyBookmarks(userID int64) []model.Bookmark {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]model.Bookmark, 0)
	for _, b := range s.bookmarks {
		if b.UserID == userID {
			res = append(res, b)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].UpdatedAt.After(res[j].UpdatedAt) })
	return res
}
