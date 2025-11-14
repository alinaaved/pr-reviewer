package model

import "time"

type TeamDB struct {
	TeamName string `gorm:"primaryKey;column:team_name"`
}

func (TeamDB) TableName() string {
	return "teams"
}

type UserDB struct {
	UserID   string `gorm:"primaryKey;column:user_id"`
	Username string `gorm:"column:username"`
	IsActive bool   `gorm:"column:is_active"`
	TeamName string `gorm:"column:team_name"`
}

func (UserDB) TableName() string { return "users" }

type PullRequestDB struct {
	ID        string     `gorm:"primaryKey;column:pull_request_id"`
	Name      string     `gorm:"column:pull_request_name"`
	AuthorID  string     `gorm:"column:author_id"`
	Status    string     `gorm:"column:status"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	MergedAt  *time.Time `gorm:"column:merged_at"`
}

func (PullRequestDB) TableName() string { return "pull_requests" }

type PRReviewerDB struct {
	PRID       string `gorm:"primaryKey;column:pr_id"`
	ReviewerID string `gorm:"column:reviewer_id"`
	Position   int16  `gorm:"primaryKey;column:position"`
}

func (PRReviewerDB) TableName() string { return "pr_reviewers" }
