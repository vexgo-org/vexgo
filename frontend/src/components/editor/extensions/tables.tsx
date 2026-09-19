import {
  EditorState,
  StateField,
  type Range,
  type Text,
} from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  EditorView,
  WidgetType,
} from "@codemirror/view";
import { syntaxTree } from "@codemirror/language";
import type { SyntaxNode } from "@lezer/common";

import { activeLineNumbers } from "./livePreview";

/* ------------------------------------------------------------------ *
 * Model
 * ------------------------------------------------------------------ */

type TableAlign = "left" | "center" | "right";

/** A styled slice of a cell, already stripped of its markdown markers. */
interface CellRun {
  text: string;
  bold?: boolean;
  italic?: boolean;
  strike?: boolean;
  code?: boolean;
  href?: string;
}

interface TableModel {
  align: TableAlign[];
  header: CellRun[][];
  rows: CellRun[][][];
}

type RunStyle = Omit<CellRun, "text">;

/**
 * Nodes that only delimit other nodes. They are skipped rather than copied so
 * a rendered cell never shows the `**`, backticks or `[]()` of its source.
 */
const MARKER_NODES = new Set([
  "EmphasisMark",
  "StrikethroughMark",
  "CodeMark",
  "LinkMark",
  "URL",
  "LinkTitle",
]);

function sameStyle(a: RunStyle, b: RunStyle): boolean {
  return (
    a.bold === b.bold &&
    a.italic === b.italic &&
    a.strike === b.strike &&
    a.code === b.code &&
    a.href === b.href
  );
}

function pushRun(out: CellRun[], text: string, style: RunStyle) {
  if (!text) return;
  const last = out[out.length - 1];
  if (last && sameStyle(last, style)) {
    last.text += text;
    return;
  }
  out.push({ text, ...style });
}

/**
 * Collects the inline content of a node as runs. Text between child nodes
 * belongs to no node of its own, so the gaps are sliced out by position.
 */
function collectRuns(
  doc: Text,
  node: SyntaxNode,
  style: RunStyle,
  out: CellRun[],
) {
  let pos = node.from;
  for (let child = node.firstChild; child; child = child.nextSibling) {
    // The gap has to be flushed before the marker check: the content of a
    // construct sits *between* its two markers, so skipping a marker without
    // flushing first would drop the text it delimits.
    if (child.from > pos) pushRun(out, doc.sliceString(pos, child.from), style);
    if (MARKER_NODES.has(child.name)) {
      pos = child.to;
      continue;
    }
    switch (child.name) {
      case "StrongEmphasis":
        collectRuns(doc, child, { ...style, bold: true }, out);
        break;
      case "Emphasis":
        collectRuns(doc, child, { ...style, italic: true }, out);
        break;
      case "Strikethrough":
        collectRuns(doc, child, { ...style, strike: true }, out);
        break;
      case "InlineCode":
        collectRuns(doc, child, { ...style, code: true }, out);
        break;
      case "Link": {
        const url = child.getChild("URL");
        collectRuns(
          doc,
          child,
          {
            ...style,
            href: url ? doc.sliceString(url.from, url.to) : undefined,
          },
          out,
        );
        break;
      }
      case "Autolink": {
        const text = doc
          .sliceString(child.from, child.to)
          .replace(/^<|>$/g, "");
        pushRun(out, text, { ...style, href: text });
        break;
      }
      case "Image": {
        // The image itself is only rendered in the document body, so a cell
        // falls back to the alt text.
        const marks = [];
        for (let c = child.firstChild; c; c = c.nextSibling) {
          if (c.name === "LinkMark") marks.push(c);
        }
        if (marks.length >= 2) {
          pushRun(out, doc.sliceString(marks[0].to, marks[1].from), style);
        }
        break;
      }
      case "Escape":
        pushRun(out, doc.sliceString(child.from + 1, child.to), style);
        break;
      default:
        pushRun(out, doc.sliceString(child.from, child.to), style);
        break;
    }
    pos = child.to;
  }
  if (pos < node.to) pushRun(out, doc.sliceString(pos, node.to), style);
}

function cellRuns(doc: Text, row: SyntaxNode): CellRun[][] {
  const cells: CellRun[][] = [];
  for (let child = row.firstChild; child; child = child.nextSibling) {
    if (child.name !== "TableCell") continue;
    const runs: CellRun[] = [];
    collectRuns(doc, child, {}, runs);
    cells.push(runs);
  }
  return cells;
}

/** Reads `:---`, `:---:` and `---:` cells off the delimiter row. */
function parseAlign(text: string): TableAlign[] {
  const align: TableAlign[] = [];
  for (const raw of text.split("|")) {
    const cell = raw.trim();
    if (!cell) continue;
    const left = cell.startsWith(":");
    const right = cell.endsWith(":");
    align.push(left && right ? "center" : right ? "right" : "left");
  }
  return align;
}

function tableModel(doc: Text, table: SyntaxNode): TableModel | null {
  const header = table.getChild("TableHeader");
  if (!header) return null;
  const delimiter = table.getChild("TableDelimiter");
  const align = delimiter
    ? parseAlign(doc.sliceString(delimiter.from, delimiter.to))
    : [];
  const rows: CellRun[][][] = [];
  for (let child = table.firstChild; child; child = child.nextSibling) {
    if (child.name === "TableRow") rows.push(cellRuns(doc, child));
  }
  return { align, header: cellRuns(doc, header), rows };
}

/* ------------------------------------------------------------------ *
 * Widget
 * ------------------------------------------------------------------ */

function appendRuns(parent: HTMLElement, runs: CellRun[]) {
  for (const run of runs) {
    const doc = parent.ownerDocument;
    let node: HTMLElement = doc.createElement("span");
    node.textContent = run.text;
    const wrap = (tag: string) => {
      const el = doc.createElement(tag);
      el.appendChild(node);
      node = el;
    };
    if (run.code) wrap("code");
    if (run.bold) wrap("strong");
    if (run.italic) wrap("em");
    if (run.strike) wrap("del");
    if (run.href) {
      const link = doc.createElement("a");
      link.href = run.href;
      link.target = "_blank";
      link.rel = "noopener";
      link.appendChild(node);
      node = link;
    }
    parent.appendChild(node);
  }
}

class TableWidget extends WidgetType {
  readonly model: TableModel;
  readonly key: string;

  constructor(model: TableModel, key: string) {
    super();
    this.model = model;
    this.key = key;
  }

  eq(other: TableWidget) {
    return other.key === this.key;
  }

  toDOM(view: EditorView) {
    const doc = document;
    const wrap = doc.createElement("div");
    wrap.className = "vexgo-md-table";

    const table = doc.createElement("table");
    const alignOf = (index: number) => this.model.align[index] ?? "left";

    const head = doc.createElement("thead");
    const headRow = doc.createElement("tr");
    this.model.header.forEach((cell, index) => {
      const th = doc.createElement("th");
      th.style.textAlign = alignOf(index);
      appendRuns(th, cell);
      headRow.appendChild(th);
    });
    head.appendChild(headRow);
    table.appendChild(head);

    if (this.model.rows.length > 0) {
      const body = doc.createElement("tbody");
      for (const row of this.model.rows) {
        const tr = doc.createElement("tr");
        row.forEach((cell, index) => {
          const td = doc.createElement("td");
          td.style.textAlign = alignOf(index);
          appendRuns(td, cell);
          tr.appendChild(td);
        });
        body.appendChild(tr);
      }
      table.appendChild(body);
    }

    wrap.appendChild(table);

    wrap.addEventListener("mousedown", (event) => {
      event.preventDefault();
      event.stopPropagation();
      if (view.state.readOnly) return;
      // The table moves with the document, so its position is read back out of
      // the DOM instead of being cached when the widget was built.
      const pos = view.posAtDOM(wrap);
      view.dispatch({
        selection: { anchor: Math.min(pos + 1, view.state.doc.length) },
        scrollIntoView: true,
      });
      view.focus();
    });

    return wrap;
  }

  ignoreEvent() {
    return false;
  }
}

/* ------------------------------------------------------------------ *
 * Extension
 * ------------------------------------------------------------------ */

function buildTables(state: EditorState): DecorationSet {
  const doc = state.doc;
  const active = activeLineNumbers(state);
  const ranges: Range<Decoration>[] = [];

  syntaxTree(state).iterate({
    enter(node) {
      if (node.name !== "Table") return;
      const first = doc.lineAt(node.from).number;
      const last = doc.lineAt(node.to).number;
      for (let n = first; n <= last; n++) {
        // A table is one block, so the whole thing stays source while the
        // cursor is anywhere inside it — revealing single rows would tear the
        // rendered table apart.
        if (active.has(n)) return false;
      }
      // A quoted table would leave its `> ` marker stranded next to the
      // rendered table, so it keeps its source form.
      for (let parent = node.node.parent; parent; parent = parent.parent) {
        if (parent.name === "Blockquote") return false;
      }
      const model = tableModel(doc, node.node);
      if (!model) return false;
      ranges.push(
        Decoration.replace({
          widget: new TableWidget(model, doc.sliceString(node.from, node.to)),
          block: true,
        }).range(node.from, node.to),
      );
      return false;
    },
  });

  return Decoration.set(ranges, true);
}

/**
 * Renders GFM tables as real `<table>` elements. This cannot live in the
 * rich-text view plugin: CodeMirror refuses block widgets and multi-line
 * replacements coming from a plugin (`EditorView.decorations` values that are
 * functions), while a state field hands over a plain decoration set.
 */
export const tableWidgets = StateField.define<DecorationSet>({
  create: (state) => buildTables(state),
  update(value, tr) {
    if (!tr.docChanged && !tr.selection) return value;
    return buildTables(tr.state);
  },
  provide: (field) => EditorView.decorations.from(field),
});
