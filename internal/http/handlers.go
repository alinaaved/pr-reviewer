package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alinaaved/pr-reviewer/internal/model"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, code, msg string, status int) {
	var resp ErrorResponse
	resp.Error.Code, resp.Error.Message = code, msg
	writeJSON(w, status, resp)
}

// Liveness
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /team/add -> 201 {team:{...}} | 400 TEAM_EXISTS
func (h *Handler) TeamAdd(w http.ResponseWriter, r *http.Request) {
	var in Team
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, "BAD_REQUEST", "invalid json", http.StatusBadRequest)
		return
	}
	if in.TeamName == "" {
		writeErr(w, "BAD_REQUEST", "team_name is required", http.StatusBadRequest)
		return
	}

	// по контракту — если уже есть, вернуть 400 TEAM_EXISTS
	var cnt int64
	if err := h.db.Model(&model.TeamDB{}).
		Where("team_name = ?", in.TeamName).Count(&cnt).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		writeErr(w, "TEAM_EXISTS", "team_name already exists", http.StatusBadRequest)
		return
	}

	// создаём команду
	if err := h.db.Create(&model.TeamDB{TeamName: in.TeamName}).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}

	// upsert участников
	for _, m := range in.Members {
		u := model.UserDB{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
			TeamName: in.TeamName,
		}
		if err := h.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"username", "is_active", "team_name"}),
		}).Create(&u).Error; err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"team": in})
}

// GET /team/get?team_name=... -> 200 (голый Team) | 404
func (h *Handler) TeamGet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("team_name")
	if name == "" {
		writeErr(w, "BAD_REQUEST", "team_name is required", http.StatusBadRequest)
		return
	}

	var exists int64
	if err := h.db.Model(&model.TeamDB{}).
		Where("team_name = ?", name).Count(&exists).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		writeErr(w, "NOT_FOUND", "team not found", http.StatusNotFound)
		return
	}

	var users []model.UserDB
	if err := h.db.Where("team_name = ?", name).Find(&users).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}

	out := Team{TeamName: name}
	for _, u := range users {
		out.Members = append(out.Members, TeamMember{
			UserID:   u.UserID,
			Username: u.Username,
			IsActive: u.IsActive,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /users/setIsActive -> 200 {user:{...}} | 404
func (h *Handler) UsersSetIsActive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, "BAD_REQUEST", "invalid json", http.StatusBadRequest)
		return
	}

	var u model.UserDB
	if err := h.db.First(&u, "user_id = ?", in.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, "NOT_FOUND", "user not found", http.StatusNotFound)
			return
		}
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}

	if err := h.db.Model(&u).Update("is_active", in.IsActive).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	u.IsActive = in.IsActive

	writeJSON(w, http.StatusOK, map[string]any{
		"user": User{
			UserID:   u.UserID,
			Username: u.Username,
			TeamName: u.TeamName,
			IsActive: u.IsActive,
		},
	})
}

// GET /users/getReview?user_id=... -> 200 { user_id, pull_requests:[...] }
func (h *Handler) UsersGetReview(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		writeErr(w, "BAD_REQUEST", "user_id is required", http.StatusBadRequest)
		return
	}

	type row struct {
		ID     string
		Name   string
		Author string
		Status string
	}
	var rows []row
	if err := h.db.
		Table("pr_reviewers AS r").
		Select("pr.pull_request_id AS id, pr.pull_request_name AS name, pr.author_id AS author, pr.status").
		Joins("JOIN pull_requests pr ON pr.pull_request_id = r.pr_id").
		Where("r.reviewer_id = ?", uid).
		Order("pr.created_at DESC").
		Scan(&rows).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}

	list := make([]PullRequestShort, 0, len(rows))
	for _, x := range rows {
		list = append(list, PullRequestShort{
			PullRequestID:   x.ID,
			PullRequestName: x.Name,
			AuthorID:        x.Author,
			Status:          x.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":       uid,
		"pull_requests": list,
	})
}

func (h *Handler) buildPR(pr model.PullRequestDB) (PullRequest, error) {
	var rows []model.PRReviewerDB
	if err := h.db.
		Where("pr_id = ?", pr.ID).
		Order("position ASC").
		Find(&rows).Error; err != nil {
		return PullRequest{}, err
	}
	assigned := make([]string, 0, len(rows))
	for _, r := range rows {
		assigned = append(assigned, r.ReviewerID)
	}
	out := PullRequest{
		PullRequestID:   pr.ID,
		PullRequestName: pr.Name,
		AuthorID:        pr.AuthorID,
		Status:          pr.Status,
		Assigned:        assigned,
		CreatedAt:       &pr.CreatedAt,
		MergedAt:        pr.MergedAt,
	}
	return out, nil
}

// POST /pullRequest/create
// 201 {pr:{...}} | 404 NOT_FOUND (нет автора/команды) | 409 PR_EXISTS
func (h *Handler) PRCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID   string `json:"pull_request_id"`
		Name string `json:"pull_request_name"`
		Auth string `json:"author_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, "BAD_REQUEST", "invalid json", http.StatusBadRequest)
		return
	}
	// 1) проверка автора
	var author model.UserDB
	if err := h.db.First(&author, "user_id = ?", in.Auth).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, "NOT_FOUND", "author not found", http.StatusNotFound)
			return
		}
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	// 2) защита от дубля PR
	var cnt int64
	if err := h.db.Model(&model.PullRequestDB{}).
		Where("pull_request_id = ?", in.ID).
		Count(&cnt).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		writeErr(w, "PR_EXISTS", "PR id already exists", http.StatusConflict)
		return
	}

	// 3) транзакция: вставка PR + назначение до двух активных ревьюверов из команды автора
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		pr := model.PullRequestDB{
			ID:       in.ID,
			Name:     in.Name,
			AuthorID: in.Auth,
			Status:   "OPEN",
		}
		if err := tx.Create(&pr).Error; err != nil {
			return err
		}

		// кандидаты: активные из команды автора, не автор; случайный порядок; максимум 2
		type cand struct{ ID string }
		var cands []cand
		if err := tx.
			Table("users").
			Select("user_id AS id").
			Where("team_name = ? AND is_active = TRUE AND user_id <> ?", author.TeamName, in.Auth).
			Order("random()").
			Limit(2).
			Scan(&cands).Error; err != nil {
			return err
		}
		for i, c := range cands {
			rec := model.PRReviewerDB{PRID: pr.ID, ReviewerID: c.ID, Position: int16(i + 1)}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}

	// 4) собрать и отдать ответ 201
	var saved model.PullRequestDB
	if err := h.db.First(&saved, "pull_request_id = ?", in.ID).Error; err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	out, err := h.buildPR(saved)
	if err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"pr": out})
}

// POST /pullRequest/merge — идемпотентно
// 200 {pr:{...}} | 404 NOT_FOUND
func (h *Handler) PRMerge(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, "BAD_REQUEST", "invalid json", http.StatusBadRequest)
		return
	}
	var pr model.PullRequestDB
	if err := h.db.First(&pr, "pull_request_id = ?", in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, "NOT_FOUND", "PR not found", http.StatusNotFound)
			return
		}
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	if pr.Status != "MERGED" { // идемпотентность
		if err := h.db.Model(&pr).
			Updates(map[string]any{"status": "MERGED", "merged_at": gorm.Expr("now()")}).
			Error; err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return
		}
		_ = h.db.First(&pr, "pull_request_id = ?", in.ID).Error
	}
	out, err := h.buildPR(pr)
	if err != nil {
		writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pr": out})
}

// POST /pullRequest/reassign
// 200 {pr:{...}, replaced_by:"uX"} |
// 404 NOT_FOUND | 409 PR_MERGED | 409 NOT_ASSIGNED | 409 NO_CANDIDATE
func (h *Handler) PRReassign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID            string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
		OldReviewerID string `json:"old_reviewer_id"` // пример в openapi использует это имя — поддержим оба
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, "BAD_REQUEST", "invalid json", http.StatusBadRequest)
		return
	}
	if in.OldUserID == "" && in.OldReviewerID != "" {
		in.OldUserID = in.OldReviewerID
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// PR существует и не MERGED?
		var pr model.PullRequestDB
		if err := tx.First(&pr, "pull_request_id = ?", in.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeErr(w, "NOT_FOUND", "PR not found", http.StatusNotFound)
				return err
			}
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}
		if pr.Status == "MERGED" {
			writeErr(w, "PR_MERGED", "cannot reassign on merged PR", http.StatusConflict)
			return errors.New("merged")
		}

		// old_user назначен?
		var slot model.PRReviewerDB
		if err := tx.First(&slot, "pr_id = ? AND reviewer_id = ?", in.ID, in.OldUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeErr(w, "NOT_ASSIGNED", "reviewer is not assigned to this PR", http.StatusConflict)
				return err
			}
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}

		// команда заменяемого
		var oldU model.UserDB
		if err := tx.First(&oldU, "user_id = ?", in.OldUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeErr(w, "NOT_FOUND", "user not found", http.StatusNotFound)
				return err
			}
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}

		// второй текущий ревьювер (чтобы не назначить его же)
		var otherIDs []string
		if err := tx.
			Table("pr_reviewers").
			Select("reviewer_id").
			Where("pr_id = ? AND reviewer_id <> ?", in.ID, in.OldUserID).
			Scan(&otherIDs).Error; err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}

		// подобрать кандидата: активный, из команды oldU, не автор, не old, не другой текущий
		query := tx.Table("users").Select("user_id AS id").
			Where("team_name = ? AND is_active = TRUE", oldU.TeamName).
			Where("user_id <> ?", in.OldUserID).
			Where("user_id <> ?", pr.AuthorID)
		if len(otherIDs) > 0 {
			query = query.Where("user_id <> ?", otherIDs[0])
		}
		var repl struct{ ID string }
		if err := query.Order("random()").Limit(1).Scan(&repl).Error; err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}
		if repl.ID == "" {
			writeErr(w, "NO_CANDIDATE", "no active replacement candidate in team", http.StatusConflict)
			return errors.New("no_candidate")
		}

		// заменить в той же позиции
		if err := tx.Model(&model.PRReviewerDB{}).
			Where("pr_id = ? AND position = ?", in.ID, slot.Position).
			Update("reviewer_id", repl.ID).Error; err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}

		// ответ
		out, err := h.buildPR(pr)
		if err != nil {
			writeErr(w, "INTERNAL", "db error", http.StatusInternalServerError)
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"pr": out, "replaced_by": repl.ID})
		return nil
	})
	if err != nil {
		return
	}
}
