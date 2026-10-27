// Agent spend as people read it: compact token counts and estimated costs.

import type { BudgetUsage, CostEstimate } from '@/api/types';

export function formatTokens(count: number): string {
  if (count < 1000) return String(count);
  if (count < 1_000_000) return `${(count / 1000).toFixed(count < 10_000 ? 1 : 0)}k`;
  return `${(count / 1_000_000).toFixed(1)}M`;
}

/** "3 calls · 12k tokens", or '' when nothing was spent. */
export function usageLabel(usage: BudgetUsage | undefined): string {
  if (!usage?.model_calls) return '';
  const calls = `${usage.model_calls} call${usage.model_calls === 1 ? '' : 's'}`;
  return `${calls} · ${formatTokens(usage.input_tokens + usage.output_tokens)} tokens`;
}

function money(value: number, currency: string | undefined): string {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency || 'USD', maximumFractionDigits: value < 1 ? 3 : 2 }).format(value);
  } catch {
    return `${value.toFixed(2)} ${currency ?? ''}`.trim();
  }
}

/** The estimated cost, "at least" a known part when some rates are missing, or ''. */
export function costLabel(cost: CostEstimate | undefined): string {
  if (!cost) return '';
  if (cost.status === 'estimated' && cost.total !== undefined) return money(cost.total, cost.currency);
  if (cost.known_subtotal > 0) return `≥ ${money(cost.known_subtotal, cost.currency)}`;
  return '';
}

/** Why a cost is not a complete estimate, in words. */
export function costCaveat(cost: CostEstimate | undefined): string {
  const reasons = cost?.reasons ?? [];
  if (reasons.includes('price_table_unavailable')) return 'Add model rates under agent_runtime.telemetry.prices to estimate costs.';
  if (reasons.includes('model_rate_unavailable')) return 'Some models have no configured rate.';
  if (reasons.includes('usage_incomplete') || reasons.includes('cache_split_unknown')) return 'Some calls did not report complete usage.';
  if (reasons.includes('legacy_usage')) return 'Older runs did not record per-model usage.';
  return '';
}
