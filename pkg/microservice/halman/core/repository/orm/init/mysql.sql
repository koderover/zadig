-- 只对独立的 zadig_ai 数据库执行，不修改 user、dex 或 aslan 数据。
CREATE TABLE IF NOT EXISTS ai_conversation (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    title VARCHAR(255) NOT NULL,
    title_source_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    last_active_at DATETIME(6) NOT NULL,
    deleted_at DATETIME(6) NULL,
    KEY idx_conversation_list (user_id, deleted_at, last_active_at, id),
    KEY idx_conversation_title (title_source_at, created_at, id),
    KEY idx_conversation_activity (last_active_at, id),
    KEY idx_conversation_deleted (deleted_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_skill (
    id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    source VARCHAR(16) CHARACTER SET ascii NOT NULL,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(32) NOT NULL,
    description VARCHAR(512) NOT NULL,
    content TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    custom_rules TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_run (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    conversation_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    status VARCHAR(32) CHARACTER SET ascii NOT NULL,
    skill_id VARCHAR(64) CHARACTER SET ascii NOT NULL DEFAULT '',
    skill_execution_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    skill_status VARCHAR(32) CHARACTER SET ascii NULL,
    active_plan_run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    context JSON NOT NULL,
    resource_scopes JSON NOT NULL,
    model_id VARCHAR(128) NOT NULL DEFAULT '',
    model_name VARCHAR(255) NOT NULL DEFAULT '',
    answer_version INT NOT NULL DEFAULT 1,
    answer_available BOOLEAN NOT NULL DEFAULT FALSE,
    question_source_run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    plan JSON NULL,
    plan_version INT NOT NULL DEFAULT 0,
    plan_status VARCHAR(32) CHARACTER SET ascii NULL,
    plan_expires_at DATETIME(6) NULL,
    usage_contribution JSON NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    elapsed_ms BIGINT NOT NULL DEFAULT 0,
    error VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    UNIQUE KEY idx_run_request (user_id, request_id),
    KEY idx_run_conversation (conversation_id, created_at, id),
    KEY idx_run_skill (skill_execution_id, created_at, id),
    KEY idx_run_created (created_at, id),
    KEY idx_run_user_status (user_id, status),
    KEY idx_run_status (status, created_at, id),
    KEY idx_run_plan_status (plan_status, created_at, id),
    KEY idx_run_question_version (question_source_run_id, answer_available, answer_version, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_feedback (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    answer_run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    answer_version INT NOT NULL,
    helpful BOOLEAN NOT NULL,
    created_at DATETIME(6) NOT NULL,
    UNIQUE KEY idx_feedback_answer (user_id, answer_run_id, answer_version),
    KEY idx_feedback_created (created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_usage_daily (
    date DATE NOT NULL,
    user_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    skill_id VARCHAR(64) CHARACTER SET ascii NOT NULL DEFAULT '',
    model_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
    model_name VARCHAR(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
    run_count BIGINT NOT NULL DEFAULT 0,
    completed_count BIGINT NOT NULL DEFAULT 0,
    terminal_count BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    elapsed_ms BIGINT NOT NULL DEFAULT 0,
    completed_elapsed_ms BIGINT NOT NULL DEFAULT 0,
    positive_count BIGINT NOT NULL DEFAULT 0,
    negative_count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date, user_id, skill_id, model_id, model_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
