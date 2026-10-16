export function findingTitle(title: string, summary = ''): string {
  const normalized = title.trim().replace(/\s+/g, ' ');
  // Saved semantic findings predate content-specific titles.
  return (
    (!normalized || /^(File|Project) analysis suggestion$/i.test(normalized)
      ? summary.trim().replace(/\s+/g, ' ')
      : normalized) || 'Untitled finding'
  );
}

export function findingName(title: string, summary = ''): string {
  const content = findingTitle(title, summary);
  const words = content.split(' ');
  let name = words.slice(0, 6).join(' ');
  const characters = Array.from(name);
  if (characters.length > (words.length > 6 ? 59 : 60)) {
    name = characters.slice(0, 59).join('');
    if (name.includes(' ')) name = name.slice(0, name.lastIndexOf(' '));
  }
  return name === content ? name : `${name.replace(/[\s.,;:!?…]+$/u, '')}…`;
}

export interface DedupeFindingItem {
  path: string;
  line?: number;
  title: string;
  name: string;
  severity?: string;
  category?: string;
  status?: string;
  target: {
    kind: string;
    id: string;
    path: string;
    source?: string;
  };
}

function normalizeFindingText(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim();
}

function isActiveOrCompletedStatus(status?: string): boolean {
  return status === 'in_progress' || status === 'fixed' || status === 'done';
}

function findingSpecificity(item: DedupeFindingItem): number {
  let score = 0;
  if (isActiveOrCompletedStatus(item.status)) score += 100;
  if (item.target.kind === 'security' || item.target.kind === 'performance') score += 20;
  if ((item.line || 0) > 0) score += 10;
  if (item.target.source === 'ai') score += 2;
  return score;
}

export function dedupeFindingItems<T extends DedupeFindingItem>(items: T[]): T[] {
  const dedicatedStagePaths = new Set<string>();
  for (const item of items) {
    if (item.target.kind === 'security' || item.target.kind === 'performance') {
      const cat = item.category || item.target.kind;
      dedicatedStagePaths.add(`${cat}:${item.path}`);
    }
  }

  const filtered = items.filter((item) => {
    if (
      item.target.kind === 'semantic' &&
      (!item.line || item.line <= 0) &&
      !isActiveOrCompletedStatus(item.status)
    ) {
      const cat = item.category || '';
      if (dedicatedStagePaths.has(`${cat}:${item.path}`)) {
        return false;
      }
    }
    return true;
  });

  const out: T[] = [];
  for (const item of filtered) {
    const cat = item.category || '';
    const normTitle = normalizeFindingText(item.title);
    const normName = normalizeFindingText(item.name);
    const duplicateIndex = out.findIndex((existing) => {
      if ((existing.category || '') !== cat || existing.path !== item.path) {
        return false;
      }
      const existingTitle = normalizeFindingText(existing.title);
      const existingName = normalizeFindingText(existing.name);
      const sameTitle =
        (normTitle !== '' && normTitle === existingTitle) ||
        (normName !== '' && normName === existingName);
      if (sameTitle) {
        return (
          existing.target.kind !== item.target.kind ||
          (existing.target.source || '') !== (item.target.source || '') ||
          (existing.severity || '') === (item.severity || '')
        );
      }
      if (
        (item.line || 0) > 0 &&
        existing.line === item.line &&
        (existing.target.kind !== item.target.kind ||
          (existing.target.source || '') !== (item.target.source || ''))
      ) {
        return true;
      }
      return false;
    });

    if (duplicateIndex === -1) {
      out.push(item);
    } else if (findingSpecificity(item) > findingSpecificity(out[duplicateIndex])) {
      out[duplicateIndex] = item;
    }
  }

  return out;
}
