CREATE TABLE teams (
    team_name TEXT PRIMARY KEY
);

CREATE TABLE users (
  user_id   TEXT PRIMARY KEY,
  username  TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE RESTRICT
);

CREATE TABLE pull_requests (
  pull_request_id   TEXT PRIMARY KEY,
  pull_request_name TEXT NOT NULL,
  author_id         TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
  status            TEXT NOT NULL CHECK (status IN ('OPEN','MERGED')) DEFAULT 'OPEN',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  merged_at         TIMESTAMPTZ
);

CREATE TABLE pr_reviewers (
  pr_id        TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
  reviewer_id  TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
  position     SMALLINT NOT NULL CHECK (position IN (1,2)),
  PRIMARY KEY (pr_id, position),
  UNIQUE (pr_id, reviewer_id)
);

CREATE INDEX idx_pr_reviewers_reviewer ON pr_reviewers(reviewer_id);
CREATE INDEX idx_pr_status ON pull_requests(status);