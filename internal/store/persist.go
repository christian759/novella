package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"novella/internal/model"
)

type persistentState struct {
	UsersByID         map[int64]model.User      `json:"users_by_id"`
	UsersByEmail      map[string]int64          `json:"users_by_email"`
	UsersByUsername   map[string]int64          `json:"users_by_username"`
	NovelsByID        map[int64]model.Novel     `json:"novels_by_id"`
	ChaptersByID      map[int64]model.Chapter   `json:"chapters_by_id"`
	ChapterIDsByNovel map[int64][]int64         `json:"chapter_ids_by_novel"`
	CommentsByID      map[int64]model.Comment   `json:"comments_by_id"`
	CommentIDsByNovel map[int64][]int64         `json:"comment_ids_by_novel"`
	Bookmarks         map[string]model.Bookmark `json:"bookmarks"`
	Sessions          map[string]int64          `json:"sessions"`
	NextUserID        int64                     `json:"next_user_id"`
	NextNovelID       int64                     `json:"next_novel_id"`
	NextChapterID     int64                     `json:"next_chapter_id"`
	NextCommentID     int64                     `json:"next_comment_id"`
}

func (s *Store) loadLocked() error {
	data, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state persistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if state.UsersByID != nil {
		s.usersByID = state.UsersByID
	}
	if state.UsersByEmail != nil {
		s.usersByEmail = state.UsersByEmail
	}
	if state.UsersByUsername != nil {
		s.usersByUsername = state.UsersByUsername
	}
	if state.NovelsByID != nil {
		s.novelsByID = state.NovelsByID
	}
	if state.ChaptersByID != nil {
		s.chaptersByID = state.ChaptersByID
	}
	if state.ChapterIDsByNovel != nil {
		s.chapterIDsByNovel = state.ChapterIDsByNovel
	}
	if state.CommentsByID != nil {
		s.commentsByID = state.CommentsByID
	}
	if state.CommentIDsByNovel != nil {
		s.commentIDsByNovel = state.CommentIDsByNovel
	}
	if state.Bookmarks != nil {
		s.bookmarks = state.Bookmarks
	}
	if state.Sessions != nil {
		s.sessions = state.Sessions
	}
	s.nextUserID = state.NextUserID
	s.nextNovelID = state.NextNovelID
	s.nextChapterID = state.NextChapterID
	s.nextCommentID = state.NextCommentID

	return nil
}

func (s *Store) persistLocked() error {
	if s.dbPath == "" {
		return nil
	}

	state := persistentState{
		UsersByID:         s.usersByID,
		UsersByEmail:      s.usersByEmail,
		UsersByUsername:   s.usersByUsername,
		NovelsByID:        s.novelsByID,
		ChaptersByID:      s.chaptersByID,
		ChapterIDsByNovel: s.chapterIDsByNovel,
		CommentsByID:      s.commentsByID,
		CommentIDsByNovel: s.commentIDsByNovel,
		Bookmarks:         s.bookmarks,
		Sessions:          s.sessions,
		NextUserID:        s.nextUserID,
		NextNovelID:       s.nextNovelID,
		NextChapterID:     s.nextChapterID,
		NextCommentID:     s.nextCommentID,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(s.dbPath, data, 0o600)
}
