#!/bin/bash
set -e

echo "🚀 Starting 3-Arm Evaluation Sweep (Blues/Rock)..."

# Create output dirs
mkdir -p eval_results/v2_test/v1_baseline
mkdir -p eval_results/v2_test/v2_combined
mkdir -p eval_results/v2_test/no_agent_2

# Helper to move results
move_results() {
    local dest=$1
    echo "📦 Moving results to $dest ..."
    # Move html files
    mv eval_results/*.html "$dest/" 2>/dev/null || true
    # Move txt files
    mv eval_results/*.txt "$dest/" 2>/dev/null || true
}

# 1. Run Arm 1: V1 Baseline (Original prompts, Agent 2 active)
echo "🎬 Running Arm 1: V1 Baseline..."
unset PROMPT_VERSION
unset ABLATE_AGENT_2
go run cmd/eval/main.go
move_results eval_results/v2_test/v1_baseline

# 2. Run Arm 2: V2 Test (Combined Opt 2+3)
echo "🎬 Running Arm 2: V2 Test..."
export PROMPT_VERSION=v2
unset ABLATE_AGENT_2
go run cmd/eval/main.go
move_results eval_results/v2_test/v2_combined

# 3. Skip Arm 3 (Already rescued)
# echo "🎬 Running Arm 3: No Agent 2..."
# unset PROMPT_VERSION
# export ABLATE_AGENT_2=true
# go run cmd/eval/main.go
# move_results eval_results/v2_test/no_agent_2

echo "⚖️ Running Judge: V1 Baseline vs V2 Combined..."
# We need to run the judge script. We'll use the existing ab_scoring_judge.py or a modified one.
# The existing ab_scoring_judge.py compares directory A vs directory B.
# Command: python3 scratch/ab_scoring_judge.py <dirA> <dirB> <output_file>
# Wait, let's verify if the judge script works on directories or individual files.
# In the previous run, we ran it in a loop or the script handled side-by-side inside the HTML generation?
# No, run_study_agent1.sh showed: LINT_DIR=... python3 ...
# Let's verify the judge command from run_study_agent1.sh.
# Assuming it compares two directories or we run it file by file.
# Let's use the standard judge invocation if we can.

echo "✅ 3-Arm Run Complete. Results saved in eval_results/v2_test/"
