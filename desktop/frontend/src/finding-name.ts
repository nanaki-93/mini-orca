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
