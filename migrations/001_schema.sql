-- DDL for metaflow_cicd (sources, projects, secrets)
-- Run this before starting the application.

-- 1. 자산 및 인증 정보 관리 테이블
CREATE TABLE IF NOT EXISTS sources (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(20) NOT NULL,
    host TEXT,
    port INTEGER,
    username VARCHAR(100),
    password_text TEXT,
    access_token TEXT,
    webhook_secret TEXT,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. 프로젝트 및 트리거 설정 테이블
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    project_name VARCHAR(100) UNIQUE NOT NULL,
    main_repo_url TEXT NOT NULL,
    target_branches TEXT[] NOT NULL,
    ci_source_name VARCHAR(100) REFERENCES sources(name),
    ci_config_path TEXT NOT NULL,
    cd_source_name VARCHAR(100) REFERENCES sources(name),
    cd_config_path TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. 파이프라인용 시크릿 정보 테이블
CREATE TABLE IF NOT EXISTS secrets (
    id SERIAL PRIMARY KEY,
    project_id INTEGER REFERENCES projects(id),
    secret_key VARCHAR(100) NOT NULL,
    secret_value TEXT NOT NULL,
    scope VARCHAR(50) DEFAULT 'prod',
    description TEXT,
    UNIQUE(project_id, secret_key, scope)
);

CREATE INDEX IF NOT EXISTS idx_projects_main_repo ON projects(main_repo_url);
