#!/usr/bin/env python3
"""
Minimal Metaflow-style CI flow for metaflow_cicd pipeline testing.
Run: python metaflow_ci.py run

In production, replace with a proper Metaflow flow (from metaflow import FlowSpec, step).
"""
import sys
import os


def main():
    print("metaflow_cicd CI flow - OK")
    # Optional: verify injected secrets (e.g. REGISTRY_ID, REGISTRY_PASSWORD)
    for key in ("REGISTRY_ID", "REGISTRY_PASSWORD", "GITHUB_TOKEN"):
        if key in os.environ:
            print(f"  {key}: [REDACTED]")
    sys.exit(0)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "run":
        main()
    else:
        print("Usage: python metaflow_ci.py run")
        sys.exit(1)
