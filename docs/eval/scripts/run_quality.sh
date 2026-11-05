#!/usr/bin/env bash
# Quality scoring for the 2026-11-05 head-to-head (plan §6.2): strip each run's own Critic notes,
# re-score every preset with one fixed Critic, and judge each challenger arm against PROD with four
# judges. Needs GEMINI_API_KEY. Usage: docs/eval/scripts/run_quality.sh [arms...]
set -euo pipefail
ROOT=eval_results/h2h
OUT=$ROOT/quality
CRITIC=${CRITIC_MODEL:-gemini-4-argon}
JUDGES=(gemini-3.1-pro-preview gemini-3.6-flash gemini-4-argon gemini-3.8-flash)
ARMS=("${@:-prod prodhigh all38low all38}")
read -r -a ARMS <<<"${ARMS[*]}"
mkdir -p "$OUT"

for arm in "${ARMS[@]}"; do
  for rep in 1 2; do
    merged=$OUT/merged/$arm/rep$rep
    mkdir -p "$merged"
    for half in a b; do
      src=$(ls -d "$ROOT/${arm}_${half}"/*/rep$rep 2>/dev/null | head -1 || true)
      [ -n "$src" ] && cp "$src"/*.html "$merged"/ 2>/dev/null || true
    done
    go run ./cmd/eval_strip_critic -in "$merged" -out "$OUT/stripped/$arm/rep$rep"
    CRITIC_MODEL=$CRITIC PROBE_DIR="$OUT/stripped/$arm/rep$rep" OUT_FILE="$OUT/critic_${arm}_rep$rep.json" \
      go run ./cmd/critic_probe > "$OUT/critic_${arm}_rep$rep.log" 2>&1
  done
done

for arm in "${ARMS[@]}"; do
  [ "$arm" = prod ] && continue
  for rep in 1 2; do
    for j in "${JUDGES[@]}"; do
      DIR_A="$OUT/stripped/prod/rep$rep" DIR_B="$OUT/stripped/$arm/rep$rep" LABEL_A=prod LABEL_B="$arm" \
        JUDGE_MODEL="$j" OUT_FILE="$OUT/judge_${arm}_vs_prod_rep${rep}_${j}.json" \
        go run ./cmd/judge_compare > "$OUT/judge_${arm}_vs_prod_rep${rep}_${j}.log" 2>&1 &
    done
    wait
  done
done
echo "done: $OUT"
