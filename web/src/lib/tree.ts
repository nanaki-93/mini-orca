// Folder trees from flat project-relative paths, as IDE explorers show them.

export interface TreeDir<T> {
  name: string;
  path: string;
  dirs: Map<string, TreeDir<T>>;
  files: { name: string; path: string; entry: T }[];
  count: number;
}

export function buildTree<T>(entries: readonly T[], pathOf: (entry: T) => string): TreeDir<T> {
  const root: TreeDir<T> = { name: '', path: '', dirs: new Map(), files: [], count: 0 };
  for (const entry of entries) {
    const path = pathOf(entry);
    if (!path) continue;
    const parts = path.split('/');
    let node = root;
    node.count++;
    for (let i = 0; i < parts.length - 1; i++) {
      const name = parts[i];
      let next = node.dirs.get(name);
      if (!next) {
        next = { name, path: parts.slice(0, i + 1).join('/'), dirs: new Map(), files: [], count: 0 };
        node.dirs.set(name, next);
      }
      node = next;
      node.count++;
    }
    node.files.push({ name: parts[parts.length - 1], path, entry });
  }
  return root;
}

export type TreeRow<T> =
  | { type: 'dir'; path: string; label: string; depth: number; open: boolean; count: number }
  | { type: 'file'; path: string; label: string; depth: number; entry: T };

/** Flattens visible rows, compacting single-child folder chains ("internal/api"). */
export function visibleRows<T>(root: TreeDir<T>, expanded: ReadonlySet<string>, depth = 0, out: TreeRow<T>[] = []): TreeRow<T>[] {
  const dirs = [...root.dirs.values()].sort((a, b) => a.name.localeCompare(b.name));
  for (let dir of dirs) {
    let label = dir.name;
    while (dir.files.length === 0 && dir.dirs.size === 1) {
      const child: TreeDir<T> = [...dir.dirs.values()][0];
      label += '/' + child.name;
      dir = child;
    }
    const open = expanded.has(dir.path);
    out.push({ type: 'dir', path: dir.path, label, depth, open, count: dir.count });
    if (open) visibleRows(dir, expanded, depth + 1, out);
  }
  for (const file of [...root.files].sort((a, b) => a.name.localeCompare(b.name))) {
    out.push({ type: 'file', path: file.path, label: file.name, depth, entry: file.entry });
  }
  return out;
}

/** Every ancestor folder of `path`, so a selected file can be revealed. */
export function ancestors(path: string): string[] {
  const parts = path.split('/');
  const out: string[] = [];
  for (let i = 1; i < parts.length; i++) out.push(parts.slice(0, i).join('/'));
  return out;
}
