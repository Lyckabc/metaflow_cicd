-- Seed data for metaflow_manager project (webhook/trigger service CI)
-- PR: Lyckabc/metaflow_manager #1 (action: synchronize → build_mode=ci)

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
    'metaflow_manager',
    'https://github.com/Lyckabc/metaflow_manager',
    ARRAY['main', 'dev', '^feature/.*', '^release-.*'],
    NULL,
    'flows/metaflow_manager.toml',
    NULL,
    'flows/metaflow_manager.toml',
    TRUE,
    'Webhook/Trigger service - GitHub webhook to Temporal ManagerWorkflow'
) ON CONFLICT (project_name) DO UPDATE SET
    main_repo_url = EXCLUDED.main_repo_url,
    target_branches = EXCLUDED.target_branches,
    ci_config_path = EXCLUDED.ci_config_path,
    cd_config_path = EXCLUDED.cd_config_path,
    updated_at = CURRENT_TIMESTAMP;
