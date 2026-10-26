// Subsequence fuzzy matching tuned for paths and command labels: consecutive
// characters, word starts and basename hits rank higher than scattered hits.

const BOUNDARY = /[\s/._\-:]/;

/** Scores `text` against `query`; returns null when not every query char appears in order. */
export function fuzzyScore(query, text) {
  const q = String(query || '').toLowerCase().replace(/\s+/g, '');
  const t = String(text || '');
  const lower = t.toLowerCase();
  if (!q) return { score: 0, positions: [] };
  const baseStart = lower.lastIndexOf('/') + 1;
  const inBase = lower.indexOf(q, baseStart);
  const exact = inBase >= 0 ? inBase : lower.indexOf(q);
  if (exact >= 0) {
    const positions = Array.from({ length: q.length }, (_, i) => exact + i);
    const atBoundary = exact === 0 || BOUNDARY.test(t[exact - 1]);
    const inBasename = exact >= baseStart;
    return { score: 1000 - (exact - (inBasename ? baseStart : 0)) + (atBoundary ? 200 : 0) + (inBasename ? 300 : 0) - (t.length - q.length) * 0.5, positions };
  }
  const positions = [];
  let score = 0;
  let from = 0;
  let streak = 0;
  for (const ch of q) {
    const index = lower.indexOf(ch, from);
    if (index < 0) return null;
    const contiguous = positions.length && index === positions[positions.length - 1] + 1;
    streak = contiguous ? streak + 1 : 0;
    score += 10 + streak * 15 + (index === 0 || BOUNDARY.test(t[index - 1]) ? 25 : 0) - Math.min(20, index - from);
    positions.push(index);
    from = index + 1;
  }
  if (positions[0] > lower.lastIndexOf('/')) score += 40;
  return { score: score - t.length * 0.2, positions };
}

/** Filters and ranks items by `key(item)`, keeping input order for ties. */
export function rankItems(items, query, key, limit = 50) {
  if (!String(query || '').trim()) return items.slice(0, limit).map(item => ({ item, positions: [] }));
  const scored = [];
  items.forEach((item, order) => {
    const match = fuzzyScore(query, key(item));
    if (match) scored.push({ item, positions: match.positions, score: match.score + (item.boost || 0), order });
  });
  scored.sort((a, b) => b.score - a.score || a.order - b.order);
  return scored.slice(0, limit);
}

/** Wraps matched positions in <mark> while escaping everything else. */
export function highlight(text, positions, escape) {
  const set = new Set(positions || []);
  let out = '';
  let open = false;
  for (let i = 0; i < text.length; i++) {
    const hit = set.has(i);
    if (hit && !open) { out += '<mark>'; open = true; }
    if (!hit && open) { out += '</mark>'; open = false; }
    out += escape(text[i]);
  }
  return out + (open ? '</mark>' : '');
}
