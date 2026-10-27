// Subsequence fuzzy matching tuned for paths and command labels: consecutive
// characters, word starts and basename hits rank higher than scattered hits.

const BOUNDARY = /[\s/._\-:]/;

export interface FuzzyMatch {
  score: number;
  positions: number[];
}

/** Scores `text` against `query`; null when not every query character appears in order. */
export function fuzzyScore(query: string, text: string): FuzzyMatch | null {
  const q = query.toLowerCase().replace(/\s+/g, '');
  const lower = text.toLowerCase();
  if (!q) return { score: 0, positions: [] };
  const baseStart = lower.lastIndexOf('/') + 1;
  const inBase = lower.indexOf(q, baseStart);
  const exact = inBase >= 0 ? inBase : lower.indexOf(q);
  if (exact >= 0) {
    const positions = Array.from({ length: q.length }, (_, i) => exact + i);
    const atBoundary = exact === 0 || BOUNDARY.test(text[exact - 1]);
    const inBasename = exact >= baseStart;
    return { score: 1000 - (exact - (inBasename ? baseStart : 0)) + (atBoundary ? 200 : 0) + (inBasename ? 300 : 0) - (text.length - q.length) * 0.5, positions };
  }
  const positions: number[] = [];
  let score = 0;
  let from = 0;
  let streak = 0;
  for (const ch of q) {
    const index = lower.indexOf(ch, from);
    if (index < 0) return null;
    const contiguous = positions.length > 0 && index === positions[positions.length - 1] + 1;
    streak = contiguous ? streak + 1 : 0;
    score += 10 + streak * 15 + (index === 0 || BOUNDARY.test(text[index - 1]) ? 25 : 0) - Math.min(20, index - from);
    positions.push(index);
    from = index + 1;
  }
  if (positions[0] > lower.lastIndexOf('/')) score += 40;
  return { score: score - text.length * 0.2, positions };
}

export interface Ranked<T> {
  item: T;
  positions: number[];
}

/** Filters and ranks items by `key(item)`, keeping input order for ties. */
export function rankItems<T>(items: readonly T[], query: string, key: (item: T) => string, limit = 50): Ranked<T>[] {
  if (!query.trim()) return items.slice(0, limit).map(item => ({ item, positions: [] }));
  const scored: (Ranked<T> & { score: number; order: number })[] = [];
  items.forEach((item, order) => {
    const match = fuzzyScore(query, key(item));
    if (match) scored.push({ item, positions: match.positions, score: match.score, order });
  });
  scored.sort((a, b) => b.score - a.score || a.order - b.order);
  return scored.slice(0, limit);
}
