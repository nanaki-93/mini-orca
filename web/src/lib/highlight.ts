// Syntax highlighting with a bounded language set. highlight.js escapes the
// source it receives, so its HTML output is safe to insert.

import hljs from 'highlight.js/lib/core';
import go from 'highlight.js/lib/languages/go';
import typescript from 'highlight.js/lib/languages/typescript';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import yaml from 'highlight.js/lib/languages/yaml';
import markdown from 'highlight.js/lib/languages/markdown';
import python from 'highlight.js/lib/languages/python';
import bash from 'highlight.js/lib/languages/bash';
import css from 'highlight.js/lib/languages/css';
import xml from 'highlight.js/lib/languages/xml';
import ini from 'highlight.js/lib/languages/ini';
import sql from 'highlight.js/lib/languages/sql';
import makefile from 'highlight.js/lib/languages/makefile';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import rust from 'highlight.js/lib/languages/rust';
import java from 'highlight.js/lib/languages/java';

const LANGUAGES = { go, typescript, javascript, json, yaml, markdown, python, bash, css, xml, ini, sql, makefile, dockerfile, rust, java };
for (const [name, lang] of Object.entries(LANGUAGES)) hljs.registerLanguage(name, lang);

const ESCAPES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, ch => ESCAPES[ch]);
}

/** Highlights a single line (diffs show lines out of their file context). */
export function highlightLine(text: string, language: string): string {
  if (!text || !hljs.getLanguage(language)) return escapeHtml(text);
  try {
    return hljs.highlight(text, { language, ignoreIllegals: true }).value;
  } catch {
    return escapeHtml(text);
  }
}

/**
 * Highlights a whole file and splits the result into lines, closing and
 * reopening spans at line breaks so each line stands alone.
 */
export function highlightLines(source: string, language: string): string[] {
  if (!hljs.getLanguage(language) || source.length > 400_000) return source.split('\n').map(escapeHtml);
  let html: string;
  try {
    html = hljs.highlight(source, { language, ignoreIllegals: true }).value;
  } catch {
    return source.split('\n').map(escapeHtml);
  }
  const lines: string[] = [];
  const open: string[] = [];
  let line = '';
  const tag = /<span class="([^"]+)">|<\/span>|\n/g;
  let last = 0;
  for (let match = tag.exec(html); match; match = tag.exec(html)) {
    line += html.slice(last, match.index);
    last = tag.lastIndex;
    if (match[0] === '\n') {
      lines.push(line + '</span>'.repeat(open.length));
      line = open.map(cls => `<span class="${cls}">`).join('');
    } else if (match[0] === '</span>') {
      open.pop();
      line += '</span>';
    } else {
      open.push(match[1]);
      line += match[0];
    }
  }
  lines.push(line + html.slice(last));
  return lines;
}
