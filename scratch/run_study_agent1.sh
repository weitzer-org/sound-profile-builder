#!/bin/bash

# Run Study for Agent 1 (Tone Historian)
# Using isolated subdirectories for outputs

echo "🚀 [Agent 1] Running Pipeline Ablation (no_agent_1)..."
# We use env vars to bypass the agent in orchestrator.go
EXPORT_VAR="ABLATE_AGENT_1=true"
eval "$EXPORT_VAR ABLATION_SUBDIR=no_agent_1 SKIP_MONOLITHIC=true go run cmd/eval/main.go"

echo "📊 [Agent 1] Linting Results..."
LINT_DIR=eval_results/ablation/no_agent_1 python3 /usr/local/google/home/benweitzer/.gemini/jetski/brain/9f82490e-b4c7-49b8-b5a2-3101c22ac5d1/scratch/lint_outputs.py > eval_results/ablation/no_agent_1/lint_results.txt || true

echo "⚖️ [Agent 1] Judging Quality (Blinded A/B)..."
ABLATION_SUBDIR=no_agent_1 go run cmd/judge/main.go > eval_results/ablation/no_agent_1/judge_results.txt || true

echo "✅ [Agent 1] Step Complete.\n"
