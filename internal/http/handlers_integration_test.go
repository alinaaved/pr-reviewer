package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/alinaaved/pr-reviewer/internal/http"
)

// Берем DSN из окружения, иначе локальный по умолчанию
func testDSN() string {
	if v := os.Getenv("DB_DSN"); v != "" {
		return v
	}
	return "postgres://app:app@localhost:5432/app?sslmode=disable"
}

func closeResp(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
}

func mustNewDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func truncateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	// порядок важен из-за FK, CASCADE чистит зависимые таблицы
	if err := db.Exec(`TRUNCATE pr_reviewers, pull_requests, users, teams RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func mustNewServer(t *testing.T, db *gorm.DB) *httptest.Server {
	t.Helper()
	h := api.NewHandler(db)
	r := chi.NewRouter()

	// маршруты как в openapi.yml
	r.Get("/healthz", h.Healthz)

	r.Post("/team/add", h.TeamAdd)
	r.Get("/team/get", h.TeamGet)

	r.Post("/users/setIsActive", h.UsersSetIsActive)
	r.Get("/users/getReview", h.UsersGetReview)

	r.Post("/pullRequest/create", h.PRCreate)
	r.Post("/pullRequest/merge", h.PRMerge)
	r.Post("/pullRequest/reassign", h.PRReassign)

	return httptest.NewServer(r)
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(http.MethodPost, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestPR_Create_Merge_Reassign(t *testing.T) {
	db := mustNewDB(t)
	truncateAll(t, db)
	srv := mustNewServer(t, db)
	defer srv.Close()

	// 1) Создаём команду из 4 активных (u1 — автор; u2,u3,u4 — кандидаты)
	team := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Carol", "is_active": true},
			{"user_id": "u4", "username": "Dave", "is_active": true},
		},
	}
	resp := postJSON(t, srv.URL+"/team/add", team)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("team/add status=%d body=%s", resp.StatusCode, string(b))
	}
	closeResp(t, resp)

	// 2) PR #1 от u1 — автоназначение 1..2 ревьюверов (u2/u3/u4), автора исключаем
	create1 := map[string]any{"pull_request_id": "pr-1", "pull_request_name": "X", "author_id": "u1"}
	resp = postJSON(t, srv.URL+"/pullRequest/create", create1)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("pr/create status=%d body=%s", resp.StatusCode, string(b))
	}
	var pr1 struct {
		PR struct {
			ID        string   `json:"pull_request_id"`
			AuthorID  string   `json:"author_id"`
			Status    string   `json:"status"`
			Assigned  []string `json:"assigned_reviewers"`
			CreatedAt string   `json:"createdAt"`
		} `json:"pr"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&pr1)
	closeResp(t, resp)
	if pr1.PR.AuthorID != "u1" || pr1.PR.Status != "OPEN" {
		t.Fatalf("unexpected PR1: %+v", pr1.PR)
	}
	if n := len(pr1.PR.Assigned); n == 0 || n > 2 {
		t.Fatalf("PR1 assigned size=%d (want 1..2)", n)
	}
	for _, id := range pr1.PR.Assigned {
		if id == "u1" {
			t.Fatalf("author assigned in PR1")
		}
	}

	// 3) merge идемпотентно
	merge := map[string]any{"pull_request_id": "pr-1"}
	resp = postJSON(t, srv.URL+"/pullRequest/merge", merge)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("merge1 status=%d body=%s", resp.StatusCode, string(b))
	}
	closeResp(t, resp)
	resp = postJSON(t, srv.URL+"/pullRequest/merge", merge)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("merge2 status=%d body=%s", resp.StatusCode, string(b))
	}
	closeResp(t, resp)

	// 4) PR #2 для позитивного reassign
	create2 := map[string]any{"pull_request_id": "pr-2", "pull_request_name": "Y", "author_id": "u1"}
	resp = postJSON(t, srv.URL+"/pullRequest/create", create2)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("pr2/create status=%d body=%s", resp.StatusCode, string(b))
	}
	var pr2 struct {
		PR struct {
			Assigned []string `json:"assigned_reviewers"`
		} `json:"pr"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&pr2)
	closeResp(t, resp)
	if len(pr2.PR.Assigned) == 0 {
		t.Fatalf("PR2 has no assigned reviewers; need at least 1 to test reassign")
	}
	old := pr2.PR.Assigned[0]

	// 5) reassign: заменить одного ревьювера на активного из команды заменяемого
	reassign := map[string]any{"pull_request_id": "pr-2", "old_user_id": old}
	resp = postJSON(t, srv.URL+"/pullRequest/reassign", reassign)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		closeResp(t, resp)
		t.Fatalf("reassign status=%d body=%s", resp.StatusCode, string(b))
	}
	var reassignResp struct {
		PR struct {
			Assigned []string `json:"assigned_reviewers"`
		} `json:"pr"`
		ReplacedBy string `json:"replaced_by"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&reassignResp)
	closeResp(t, resp)

	if reassignResp.ReplacedBy == "" {
		t.Fatalf("reassign: replaced_by is empty")
	}
	// replaced_by не должен совпадать со старым
	if reassignResp.ReplacedBy == old {
		t.Fatalf("reassign: replaced_by equals old")
	}
	// replaced_by должен быть среди назначенных после замены
	found := false
	for _, id := range reassignResp.PR.Assigned {
		if id == reassignResp.ReplacedBy {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("reassign: replaced_by not in new assigned list")
	}
}
