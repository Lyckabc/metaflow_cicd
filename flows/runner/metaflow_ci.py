#!/usr/bin/env python3
"""
Metaflow-style CI flow for metaflow_cicd pipeline testing.
Run: python metaflow_ci.py run

Invoked by Temporal Runner per flows/metaflow-ci.toml [build] command.
In production, replace with a proper Metaflow flow (from metaflow import FlowSpec, step).

Flow: pre_build (pip install) -> this run -> on success, cicd worker performs:
  registry login -> docker build -> docker push (per [registry] in metaflow-ci.toml)
"""
import os
import sys


def main():
    print("metaflow_cicd CI flow - OK")
    # Verify injected secrets (from [secrets_mapping] in metaflow-ci.toml)
    for key in ("REGISTRY_ID", "REGISTRY_PASSWORD", "GITHUB_TOKEN", "DB_HOST", "LOG_LEVEL"):
        if key in os.environ:
            if "PASSWORD" in key or "TOKEN" in key:
                print(f"  {key}: [REDACTED]")
            else:
                print(f"  {key}: {os.environ[key]}")
    sys.exit(0)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "run":
        main()
    else:
        print("Usage: python metaflow_ci.py run")
        sys.exit(1)

