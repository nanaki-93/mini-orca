// Builds a folder tree from flat project-relative paths for the explorer.

/** Returns a root node {name:'', path:'', dirs: Map, files: []} sorted for display. */
export function buildTree(entries, pathOf = entry => entry.path) {
  const root = { name: '', path: '', dirs: new Map(), files: [], count: 0 };
  for (const entry of entries) {
    const path = pathOf(entry);
    if (!path) continue;
    const parts = path.split('/');
    let node = root;
    node.count++;
    for (let i = 0; i < parts.length - 1; i++) {
      const name = parts[i];
      if (!node.dirs.has(name)) node.dirs.set(name, { name, path: parts.slice(0, i + 1).join('/'), dirs: new Map(), files: [], count: 0 });
      node = node.dirs.get(name);
      node.count++;
    }
    node.files.push({ name: parts[parts.length - 1], path, entry });
  }
  return root;
}

/**
 * Flattens the tree into visible rows honoring expanded folders. Single-child
 * folder chains are compacted ("internal/api") as IDEs do.
 */
export function visibleRows(root, expanded, depth = 0, out = []) {
  const dirs = [...root.dirs.values()].sort((a, b) => a.name.localeCompare(b.name));
  for (let dir of dirs) {
    let label = dir.name;
    while (dir.files.length === 0 && dir.dirs.size === 1) {
      const child = [...dir.dirs.values()][0];
      label += '/' + child.name;
      dir = child;
    }
    const open = expanded.has(dir.path);
    out.push({ type: 'dir', path: dir.path, label, depth, open, count: dir.count });
    if (open) visibleRows(dir, expanded, depth + 1, out);
  }
  const files = [...root.files].sort((a, b) => a.name.localeCompare(b.name));
  for (const file of files) out.push({ type: 'file', path: file.path, label: file.name, depth, entry: file.entry });
  return out;
}

/** Returns every ancestor folder of `path` so a selected file can be revealed. */
export function ancestors(path) {
  const parts = String(path || '').split('/');
  const out = [];
  for (let i = 1; i < parts.length; i++) out.push(parts.slice(0, i).join('/'));
  return out;
}
