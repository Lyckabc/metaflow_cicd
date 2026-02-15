-- Seed data for metaflow_cicd project (self-CI/CD)
-- Adjust main_repo_url to match your actual Git URL.

INSERT INTO projects (
    project_name,
    main_repo_url,
    target_branches,
    ci_source_name,
    ci_config_path,
    cd_source_name,
    cd_config_path,
    is_active,
    description
) VALUES (
    'metaflow_cicd',
    'https://github.com/Lyckabc/metaflow_cicd',
    ARRAY['main', 'dev', '^feature/.*', '^release-.*'],
    NULL,
    'flows/metaflow-ci.toml',
    NULL,
    'flows/metaflow-ci.toml',
    TRUE,
    'Temporal-based CI/CD pipeline for Metaflow'
) ON CONFLICT (project_name) DO UPDATE SET
    main_repo_url = EXCLUDED.main_repo_url,
    target_branches = EXCLUDED.target_branches,
    ci_config_path = EXCLUDED.ci_config_path,
    cd_config_path = EXCLUDED.cd_config_path,
    updated_at = CURRENT_TIMESTAMP;
