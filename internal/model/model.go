package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordSalt string    `json:"-"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type NovelStatus string

const (
	NovelDraft     NovelStatus = "draft"
	NovelPublished NovelStatus = "published"
)

type Novel struct {
	ID          int64       `json:"id"`
	AuthorID    int64       `json:"author_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Genre       string      `json:"genre"`
	Status      NovelStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Chapter struct {
	ID        int64     `json:"id"`
	NovelID   int64     `json:"novel_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	NovelID   int64     `json:"novel_id"`
	ChapterID *int64    `json:"chapter_id,omitempty"`
	UserID    int64     `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Bookmark struct {
	UserID     int64     `json:"user_id"`
	NovelID    int64     `json:"novel_id"`
	ChapterID  *int64    `json:"chapter_id,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	ChapterPos *int      `json:"chapter_position,omitempty"`
}
