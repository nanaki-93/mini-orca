import { renderMermaidSVG } from 'beautiful-mermaid';

export function renderDiagram(source: string): { url?: string; error?: string } {
  if (source.length > 16000 || source.split('\n').length > 160)
    return { error: 'Diagram is too large to render.' };
  try {
    const svg = renderMermaidSVG(source, {
      bg: '#101b14',
      fg: '#f1fff4',
      accent: '#5cfa8b',
      font: 'sans-serif',
      padding: 24,
    });
    // SVG images cannot run scripts or activate model-authored links.
    return { url: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}` };
  } catch {
    return { error: 'Diagram preview unavailable' };
  }
}
