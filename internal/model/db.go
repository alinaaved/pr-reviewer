// Package model содержит GORM-модели (маппинг на таблицы БД)
package model

import "time"

// TeamDB маппится на таблицу teams
type TeamDB struct {
	TeamName string `gorm:"primaryKey;column:team_name"`
}

// TableName возвращает имя таблицы для TeamDB
func (TeamDB) TableName() string {
	return "teams"
}

// UserDB маппится на таблицу users
type UserDB struct {
	UserID   string `gorm:"primaryKey;column:user_id"`
	Username string `gorm:"column:username"`
	IsActive bool   `gorm:"column:is_active"`
	TeamName string `gorm:"column:team_name"`
}

// TableName возвращает имя таблицы для UserDB
func (UserDB) TableName() string { return "users" }

// PullRequestDB маппится на таблицу pull_requests
type PullRequestDB struct {
	ID        string     `gorm:"primaryKey;column:pull_request_id"`
	Name      string     `gorm:"column:pull_request_name"`
	AuthorID  string     `gorm:"column:author_id"`
	Status    string     `gorm:"column:status"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	MergedAt  *time.Time `gorm:"column:merged_at"`
}

// TableName возвращает имя таблицы для PullRequestDB
func (PullRequestDB) TableName() string { return "pull_requests" }

// PRReviewerDB маппится на таблицу pr_reviewers (слоты ревьюверов)
type PRReviewerDB struct {
	PRID       string `gorm:"primaryKey;column:pr_id"`
	ReviewerID string `gorm:"column:reviewer_id"`
	Position   int16  `gorm:"primaryKey;column:position"`
}

// TableName возвращает имя таблицы для PRReviewerDB
func (PRReviewerDB) TableName() string { return "pr_reviewers" }
