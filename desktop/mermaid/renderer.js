import './runtime.js';
import { renderMermaidSVG } from 'beautiful-mermaid';

// Skia consumes SVG presentation attributes, not CSS variables or web fonts.
export function render(source, paletteJson) {
  const p = JSON.parse(paletteJson);
  const colors = {
    bg: p.bg, fg: p.fg,
    '_text': p.fg, '_text-sec': p.muted, '_text-muted': p.muted,
    '_text-faint': p.muted, '_line': p.line, '_arrow': p.accent,
    '_node-fill': p.surface, '_node-stroke': p.border,
    '_group-fill': p.bg, '_group-hdr': p.surface,
    '_inner-stroke': p.border, '_key-badge': p.surface,
  };
  return renderMermaidSVG(source, { ...p, font: 'SansSerif', padding: 24, transparent: true })
    .replace(/<style>[\s\S]*?<\/style>/g, '')
    .replace(/ style="[^"]*"/g, '')
    .replace(/var\(--([\w-]+)\)/g, (_, name) => {
      if (!(name in colors)) throw new Error(`Unknown diagram color: ${name}`);
      return colors[name];
    })
    .replace(/<text /g, '<text font-family="sans-serif" ');
}
