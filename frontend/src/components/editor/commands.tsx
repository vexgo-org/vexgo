import type { ChangeSpec, EditorState, Line } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { redoDepth, undoDepth } from "@codemirror/commands";

/* ------------------------------------------------------------------ *
 * Inline formatting
 * ------------------------------------------------------------------ */

/**
 * Toggles a symmetric inline marker (`**`, `*`, `~~`, `` ` ``) around the
 * selection, unwrapping it when the selection is already wrapped.
 */
export function toggleInline(view: EditorView, marker: string): boolean {
  const { state } = view;
  const range = state.selection.main;
  const selected = state.sliceDoc(range.from, range.to);
  const before = state.sliceDoc(
    Math.max(0, range.from - marker.length),
    range.from,
  );
  const after = state.sliceDoc(range.to, range.to + marker.length);

  if (
    selected.length >= marker.length * 2 &&
    selected.startsWith(marker) &&
    selected.endsWith(marker)
  ) {
    const inner = selected.slice(marker.length, -marker.length);
    view.dispatch({
      changes: { from: range.from, to: range.to, insert: inner },
      selection: { anchor: range.from, head: range.from + inner.length },
    });
    return true;
  }

  if (before === marker && after === marker) {
    view.dispatch({
      changes: [
        { from: range.from - marker.length, to: range.from },
        { from: range.to, to: range.to + marker.length },
      ],
      selection: { anchor: range.from - marker.length },
    });
    return true;
  }

  if (range.empty) {
    view.dispatch({
      changes: { from: range.from, insert: marker + marker },
      selection: { anchor: range.from + marker.length },
    });
    return true;
  }

  view.dispatch({
    changes: {
      from: range.from,
      to: range.to,
      insert: marker + selected + marker,
    },
    selection: {
      anchor: range.from + marker.length,
      head: range.to + marker.length,
    },
  });
  return true;
}

export const toggleBold = (view: EditorView) => toggleInline(view, "**");
export const toggleItalic = (view: EditorView) => toggleInline(view, "*");
export const toggleStrike = (view: EditorView) => toggleInline(view, "~~");
export const toggleInlineCode = (view: EditorView) => toggleInline(view, "`");

/* ------------------------------------------------------------------ *
 * Line-level formatting
 * ------------------------------------------------------------------ */

function selectedLines(state: EditorState): Line[] {
  const lines: Line[] = [];
  const seen = new Set<number>();
  for (const range of state.selection.ranges) {
    const first = state.doc.lineAt(range.from).number;
    const last = state.doc.lineAt(range.to).number;
    for (let n = first; n <= last; n++) {
      if (seen.has(n)) continue;
      seen.add(n);
      lines.push(state.doc.line(n));
    }
  }
  return lines;
}

function apply(view: EditorView, changes: ChangeSpec[]) {
  if (changes.length === 0) return;
  view.dispatch({ changes });
}

const HEADING = /^#{1,6}\s+/;
const QUOTE = /^>\s?/;
const ANY_LIST = /^(?:[-*+]\s+(?:\[[ xX]\]\s+)?|\d+\.\s+)/;

/** Applies `#`…`######`, replacing whatever heading level was there before. */
export function setHeading(view: EditorView, level: number): boolean {
  const prefix = `${"#".repeat(level)} `;
  const lines = selectedLines(view.state);
  const texts = lines.map((line) => view.state.sliceDoc(line.from, line.to));
  const allMatch = texts.every((text) => text.startsWith(prefix));

  apply(
    view,
    lines.map((line, index) => {
      if (allMatch) {
        return { from: line.from, to: line.from + prefix.length, insert: "" };
      }
      return {
        from: line.from,
        to: line.to,
        insert: prefix + texts[index].replace(HEADING, ""),
      };
    }),
  );
  return true;
}

/** Strips headings, quotes and list markers, leaving a plain paragraph. */
export function setParagraph(view: EditorView): boolean {
  const lines = selectedLines(view.state);
  apply(
    view,
    lines.map((line) => ({
      from: line.from,
      to: line.to,
      insert: view.state
        .sliceDoc(line.from, line.to)
        .replace(HEADING, "")
        .replace(QUOTE, "")
        .replace(ANY_LIST, ""),
    })),
  );
  return true;
}

function toggleLinePrefix(
  view: EditorView,
  marker: RegExp,
  strip: RegExp,
  prefix: (index: number) => string,
) {
  const lines = selectedLines(view.state);
  const texts = lines.map((line) => view.state.sliceDoc(line.from, line.to));
  const allMatch = texts.every((text) => marker.test(text));

  apply(
    view,
    lines.map((line, index) => ({
      from: line.from,
      to: line.to,
      insert: allMatch
        ? texts[index].replace(strip, "")
        : prefix(index) + texts[index].replace(strip, ""),
    })),
  );
}

export function toggleQuote(view: EditorView): boolean {
  toggleLinePrefix(view, /^>\s?/, QUOTE, () => "> ");
  return true;
}

export function toggleBulletList(view: EditorView): boolean {
  toggleLinePrefix(view, /^[-*+]\s+(?!\[)/, ANY_LIST, () => "- ");
  return true;
}

export function toggleOrderedList(view: EditorView): boolean {
  toggleLinePrefix(view, /^\d+\.\s+/, ANY_LIST, (index) => `${index + 1}. `);
  return true;
}

export function toggleTaskList(view: EditorView): boolean {
  toggleLinePrefix(view, /^[-*+]\s+\[[ xX]\]\s+/, ANY_LIST, () => "- [ ] ");
  return true;
}

/* ------------------------------------------------------------------ *
 * Block insertion
 * ------------------------------------------------------------------ */

/**
 * Inserts a block construct on lines of its own and leaves the cursor on the
 * line below it. Putting the cursor elsewhere would keep the new block on an
 * "active" line, where the live preview deliberately shows source instead of
 * the rendered result.
 */
function insertBlock(
  view: EditorView,
  body: string,
  cursorOffsetInBody?: number,
) {
  const { state } = view;
  const range = state.selection.main;
  const before =
    range.from > 0 ? state.sliceDoc(range.from - 1, range.from) : "\n";
  const prefix = before === "\n" ? "" : "\n";
  const insert = `${prefix}${body}\n`;
  const anchor =
    range.from +
    prefix.length +
    (cursorOffsetInBody === undefined ? body.length + 1 : cursorOffsetInBody);
  view.dispatch({
    changes: { from: range.from, to: range.to, insert },
    selection: { anchor },
    scrollIntoView: true,
  });
}

export function insertCodeBlock(view: EditorView): boolean {
  const { state } = view;
  const range = state.selection.main;
  const selected = state.sliceDoc(range.from, range.to);
  // Offset 3 lands at the end of the opening fence, ready for a language.
  insertBlock(view, `\`\`\`\n${selected}\n\`\`\``, 3);
  return true;
}

export function insertHorizontalRule(view: EditorView): boolean {
  insertBlock(view, "---");
  return true;
}

export function insertTable(view: EditorView, header: string): boolean {
  const columns = [1, 2, 3].map((n) => `${header} ${n}`);
  const body = [
    `| ${columns.join(" | ")} |`,
    `| ${columns.map(() => "---").join(" | ")} |`,
    `| ${columns.map(() => "  ").join(" | ")} |`,
  ].join("\n");
  insertBlock(view, body);
  return true;
}

export function insertLink(
  view: EditorView,
  url: string,
  label?: string,
): boolean {
  const range = view.state.selection.main;
  const text = label || view.state.sliceDoc(range.from, range.to) || url;
  const insert = `[${text}](${url})`;
  view.dispatch({
    changes: { from: range.from, to: range.to, insert },
    selection: { anchor: range.from + insert.length },
  });
  return true;
}

export function insertImage(
  view: EditorView,
  url: string,
  alt: string,
): boolean {
  insertBlock(view, `![${alt}](${url})`);
  return true;
}

/** Rewrites an existing `![alt](url)` in place. */
export function replaceImage(
  view: EditorView,
  from: number,
  to: number,
  url: string,
  alt: string,
): boolean {
  const insert = `![${alt}](${url})`;
  view.dispatch({
    changes: { from, to, insert },
    selection: { anchor: from + insert.length },
  });
  return true;
}

/** Removes an image, cleaning up the line it occupied when left empty. */
export function removeImage(
  view: EditorView,
  from: number,
  to: number,
): boolean {
  const { state } = view;
  const line = state.doc.lineAt(from);
  const rest = state.sliceDoc(line.from, from) + state.sliceDoc(to, line.to);
  if (rest.trim() === "") {
    const end = line.to < state.doc.length ? line.to + 1 : line.to;
    view.dispatch({ changes: { from: line.from, to: end, insert: "" } });
    return true;
  }
  view.dispatch({ changes: { from, to, insert: "" } });
  return true;
}

/** Rewrites the info string of a fenced code block. */
export function setCodeLanguage(
  view: EditorView,
  infoFrom: number,
  infoTo: number,
  language: string,
): boolean {
  const { state } = view;
  const current = state.sliceDoc(infoFrom, infoTo);
  if (current.trim() === language.trim()) return true;
  const insert = infoTo > infoFrom ? language : language ? ` ${language}` : "";
  view.dispatch({ changes: { from: infoFrom, to: infoTo, insert } });
  return true;
}

/* ------------------------------------------------------------------ *
 * History
 * ------------------------------------------------------------------ */

export function canUndo(state: EditorState): boolean {
  return undoDepth(state) > 0;
}

export function canRedo(state: EditorState): boolean {
  return redoDepth(state) > 0;
}
