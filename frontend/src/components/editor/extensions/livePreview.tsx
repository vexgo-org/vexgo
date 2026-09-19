import {
  Decoration,
  type DecorationSet,
  EditorView,
  ViewPlugin,
  type ViewUpdate,
  WidgetType,
} from "@codemirror/view";
import type { EditorState, Line, Range } from "@codemirror/state";
import { syntaxTree } from "@codemirror/language";
import type { SyntaxNode } from "@lezer/common";

import { editorCallbacks } from "./config";
import { CODE_LANGUAGES } from "../codeLanguages";

/* ------------------------------------------------------------------ *
 * Decorations
 * ------------------------------------------------------------------ */

/** Hides a range of source markers without leaving anything behind. */
const hidden = Decoration.replace({});

class BulletWidget extends WidgetType {
  eq() {
    return true;
  }

  toDOM() {
    const span = document.createElement("span");
    span.className = "vexgo-md-bullet";
    span.textContent = "•";
    return span;
  }
}

const bullet = Decoration.replace({ widget: new BulletWidget() });

const headingLines: Record<number, Decoration> = {
  1: Decoration.line({ class: "cm-md-h1" }),
  2: Decoration.line({ class: "cm-md-h2" }),
  3: Decoration.line({ class: "cm-md-h3" }),
  4: Decoration.line({ class: "cm-md-h4" }),
  5: Decoration.line({ class: "cm-md-h5" }),
  6: Decoration.line({ class: "cm-md-h6" }),
};

const quoteLine = Decoration.line({ class: "cm-md-quote" });
const codeLine = Decoration.line({ class: "cm-md-code-line" });
const fenceLine = Decoration.line({ class: "cm-md-code-fence" });
const fenceLineLast = Decoration.line({
  class: "cm-md-code-fence cm-md-code-fence-last",
});
const fenceHiddenLine = Decoration.line({ class: "cm-md-code-fence-hidden" });
const fenceHiddenLineLast = Decoration.line({
  class: "cm-md-code-fence-hidden cm-md-code-fence-last",
});
const hrLine = Decoration.line({ class: "cm-md-hr" });
const collapsedLine = Decoration.line({ class: "cm-md-collapsed" });
const imageLine = Decoration.line({ class: "cm-md-image-line" });
const inlineCodeMark = Decoration.mark({ class: "cm-md-inline-code" });
const listMarkMark = Decoration.mark({ class: "cm-md-list-mark" });

/* ------------------------------------------------------------------ *
 * Widgets
 * ------------------------------------------------------------------ */

class CheckboxWidget extends WidgetType {
  readonly checked: boolean;

  constructor(checked: boolean) {
    super();
    this.checked = checked;
  }

  eq(other: CheckboxWidget) {
    return other.checked === this.checked;
  }

  toDOM(view: EditorView) {
    const wrap = document.createElement("span");
    wrap.className = `vexgo-md-checkbox${this.checked ? " is-checked" : ""}`;
    wrap.setAttribute("role", "checkbox");
    wrap.setAttribute("aria-checked", String(this.checked));

    const box = document.createElement("span");
    box.className = "vexgo-md-checkbox-box";
    wrap.appendChild(box);

    wrap.addEventListener("mousedown", (event) => {
      event.preventDefault();
      event.stopPropagation();
      if (view.state.readOnly) return;
      // Read the marker back out of the document so the widget can never
      // toggle a stale range after the text around it shifted.
      const pos = view.posAtDOM(wrap);
      const current = view.state.sliceDoc(pos, pos + 3);
      if (!/^\[[ xX]\]$/.test(current)) return;
      const next = current.toLowerCase() === "[x]" ? "[ ]" : "[x]";
      view.dispatch({ changes: { from: pos, to: pos + 3, insert: next } });
    });

    return wrap;
  }

  ignoreEvent() {
    return false;
  }
}

class ImageWidget extends WidgetType {
  readonly url: string;
  readonly alt: string;
  readonly from: number;
  readonly to: number;

  constructor(url: string, alt: string, from: number, to: number) {
    super();
    this.url = url;
    this.alt = alt;
    this.from = from;
    this.to = to;
  }

  eq(other: ImageWidget) {
    return (
      other.url === this.url &&
      other.alt === this.alt &&
      other.from === this.from &&
      other.to === this.to
    );
  }

  toDOM(view: EditorView) {
    const wrap = document.createElement("span");
    wrap.className = "vexgo-md-image";
    wrap.title = this.alt || this.url;

    const img = document.createElement("img");
    img.src = this.url;
    img.alt = this.alt;
    img.draggable = false;
    img.addEventListener("error", () => {
      img.remove();
      wrap.classList.add("is-broken");
      const fallback = document.createElement("span");
      fallback.className = "vexgo-md-image-broken";
      fallback.textContent = this.alt || this.url;
      wrap.appendChild(fallback);
    });
    wrap.appendChild(img);

    wrap.addEventListener("mousedown", (event) => {
      event.preventDefault();
      event.stopPropagation();
      if (view.state.readOnly) return;
      const { state } = view;
      if (!state.sliceDoc(this.from, this.to).startsWith("![")) return;
      state.facet(editorCallbacks).onImageClick?.({
        from: this.from,
        to: this.to,
        url: this.url,
        alt: this.alt,
      });
    });

    return wrap;
  }

  ignoreEvent() {
    return false;
  }
}

class CodeLanguageWidget extends WidgetType {
  readonly language: string;
  readonly infoFrom: number;
  readonly infoTo: number;

  constructor(language: string, infoFrom: number, infoTo: number) {
    super();
    this.language = language;
    this.infoFrom = infoFrom;
    this.infoTo = infoTo;
  }

  eq(other: CodeLanguageWidget) {
    return (
      other.language === this.language &&
      other.infoFrom === this.infoFrom &&
      other.infoTo === this.infoTo
    );
  }

  toDOM(view: EditorView) {
    const labels = view.state.facet(editorCallbacks).labels;

    const select = document.createElement("select");
    select.className = "vexgo-md-lang";
    select.title = labels?.codeLanguage ?? "Code language";

    const add = (value: string, label: string) => {
      const option = document.createElement("option");
      option.value = value;
      option.textContent = label;
      select.appendChild(option);
    };
    add("", labels?.plainText ?? "Plain text");
    for (const item of CODE_LANGUAGES) add(item.value, item.label);
    // Preserve info strings this list does not know about (e.g. `friends`).
    if (
      this.language &&
      !CODE_LANGUAGES.some((item) => item.value === this.language)
    ) {
      add(this.language, this.language);
    }
    select.value = this.language;

    select.addEventListener("mousedown", (event) => event.stopPropagation());
    select.addEventListener("change", () => {
      const { state } = view;
      if (state.sliceDoc(this.infoFrom, this.infoTo) !== this.language) return;
      const value = select.value;
      const insert =
        this.infoTo > this.infoFrom ? value : value ? ` ${value}` : "";
      view.dispatch({
        changes: { from: this.infoFrom, to: this.infoTo, insert },
      });
    });

    return select;
  }

  ignoreEvent() {
    return false;
  }
}

/* ------------------------------------------------------------------ *
 * Decoration builder
 * ------------------------------------------------------------------ */

function marksOf(node: SyntaxNode, name: string): SyntaxNode[] {
  const found: SyntaxNode[] = [];
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.name === name) found.push(child);
  }
  return found;
}

interface Interval {
  from: number;
  to: number;
}

/**
 * Line numbers holding a cursor. Those lines render as source so their markers
 * stay editable, every other line is rendered — the rule the rich mode is
 * built on, shared with the table renderer.
 */
export function activeLineNumbers(state: EditorState): Set<number> {
  const lines = new Set<number>();
  for (const range of state.selection.ranges) {
    const first = state.doc.lineAt(range.from).number;
    const last = state.doc.lineAt(range.to).number;
    for (let n = first; n <= last; n++) lines.add(n);
  }
  return lines;
}

/**
 * Removes the parts of `intervals` covered by `blockers`, then merges what is
 * left. Nested constructs can ask for overlapping hides (a fenced block inside
 * a blockquote, say), and CodeMirror refuses to render overlapping replace
 * decorations.
 */
function mergeHides(intervals: Interval[], blockers: Interval[]): Interval[] {
  let parts = intervals.filter((interval) => interval.to > interval.from);
  for (const blocker of blockers) {
    const next: Interval[] = [];
    for (const part of parts) {
      if (blocker.to <= part.from || blocker.from >= part.to) {
        next.push(part);
        continue;
      }
      if (blocker.from > part.from) {
        next.push({ from: part.from, to: blocker.from });
      }
      if (blocker.to < part.to) {
        next.push({ from: blocker.to, to: part.to });
      }
    }
    parts = next;
  }

  parts.sort((a, b) => a.from - b.from || a.to - b.to);
  const merged: Interval[] = [];
  for (const part of parts) {
    const last = merged[merged.length - 1];
    if (last && part.from <= last.to) {
      last.to = Math.max(last.to, part.to);
    } else {
      merged.push({ ...part });
    }
  }
  return merged;
}

interface BuiltDecorations {
  decorations: DecorationSet;
  atomic: DecorationSet;
}

function build(view: EditorView): BuiltDecorations {
  const { state } = view;
  const doc = state.doc;
  const styled: Range<Decoration>[] = [];
  const hides: Interval[] = [];
  const widgets: { range: Interval; deco: Decoration }[] = [];

  /* Lines holding a cursor are rendered as source so the markers stay
     editable; every other line is rendered. */
  const activeLines = activeLineNumbers(state);
  const isActive = (pos: number) => activeLines.has(doc.lineAt(pos).number);
  const isRangeActive = (from: number, to: number) => {
    const first = doc.lineAt(from).number;
    const last = doc.lineAt(to).number;
    for (let n = first; n <= last; n++) if (activeLines.has(n)) return true;
    return false;
  };

  const hide = (from: number, to: number) => {
    if (to > from) hides.push({ from, to });
  };
  const replaceWith = (deco: Decoration, from: number, to: number) => {
    if (to > from) widgets.push({ range: { from, to }, deco });
  };
  const decorate = (from: number, to: number, deco: Decoration) => {
    if (to > from) styled.push(deco.range(from, to));
  };
  // Nested constructs (e.g. `> > quote`) can report the same line twice, and
  // CodeMirror rejects duplicate line decorations.
  const decoratedLines = new Map<number, Decoration[]>();
  const decorateLine = (line: Line, deco: Decoration) => {
    const seen = decoratedLines.get(line.number);
    if (seen) {
      if (seen.includes(deco)) return;
      seen.push(deco);
    } else {
      decoratedLines.set(line.number, [deco]);
    }
    styled.push(deco.range(line.from));
  };

  const { from, to } = view.viewport;

  syntaxTree(state).iterate({
    from,
    to,
    enter(node) {
      const syntax = node.node;

      // A rendered table replaces its whole source, so nothing inside it may
      // be decorated: those hides would overlap the table widget. While the
      // cursor is inside the table it stays source and is handled normally.
      if (node.name === "Table" && !isRangeActive(node.from, node.to)) {
        return false;
      }

      switch (node.name) {
        case "ATXHeading1":
        case "ATXHeading2":
        case "ATXHeading3":
        case "ATXHeading4":
        case "ATXHeading5":
        case "ATXHeading6": {
          const level = Number(node.name.slice(-1));
          const line = doc.lineAt(node.from);
          decorateLine(line, headingLines[level]);
          if (isActive(node.from)) break;
          const mark = syntax.getChild("HeaderMark");
          if (!mark) break;
          const end =
            doc.sliceString(mark.to, mark.to + 1) === " "
              ? mark.to + 1
              : mark.to;
          hide(mark.from, end);
          break;
        }

        case "SetextHeading1":
        case "SetextHeading2": {
          const level = node.name === "SetextHeading1" ? 1 : 2;
          decorateLine(doc.lineAt(node.from), headingLines[level]);
          // The `===` underline is the heading's last line; collapse it.
          const underline = doc.lineAt(node.to);
          if (isActive(underline.from)) break;
          decorateLine(underline, collapsedLine);
          hide(underline.from, underline.to);
          break;
        }

        case "QuoteMark": {
          const line = doc.lineAt(node.from);
          decorateLine(line, quoteLine);
          if (isActive(node.from)) break;
          const end =
            doc.sliceString(node.to, node.to + 1) === " "
              ? node.to + 1
              : node.to;
          hide(node.from, end);
          break;
        }

        case "HorizontalRule": {
          // The rule is drawn by collapsing the line, so it has to be skipped
          // entirely while the cursor is on it — otherwise the `---` would be
          // rendered at zero font size and could neither be read nor edited.
          if (isActive(node.from)) break;
          const line = doc.lineAt(node.from);
          decorateLine(line, hrLine);
          hide(node.from, line.to);
          break;
        }

        case "ListMark": {
          const item = syntax.parent;
          if (!item || item.getChild("Task")) break;
          const list = item.parent;
          if (!list) break;
          if (list.name === "BulletList") {
            if (!isActive(node.from)) {
              replaceWith(bullet, node.from, node.to);
            }
          } else {
            decorate(node.from, node.to, listMarkMark);
          }
          break;
        }

        case "TaskMarker": {
          const item = syntax.parent?.parent;
          const listMark = item?.getChild("ListMark");
          const checked = doc
            .sliceString(node.from, node.to)
            .toLowerCase()
            .includes("x");
          if (isActive(node.from)) break;
          if (listMark) hide(listMark.from, node.from);
          replaceWith(
            Decoration.replace({ widget: new CheckboxWidget(checked) }),
            node.from,
            node.to,
          );
          break;
        }

        case "FencedCode": {
          const first = doc.lineAt(node.from);
          let last = doc.lineAt(node.to);
          if (last.number > first.number && last.from === node.to) {
            last = doc.line(last.number - 1);
          }
          const active = isRangeActive(first.from, last.to);
          const lastIsFence = last.number !== first.number;

          for (let n = first.number; n <= last.number; n++) {
            const line = doc.line(n);
            if (n === first.number) {
              decorateLine(line, active ? fenceLine : fenceHiddenLine);
            } else if (n === last.number && lastIsFence) {
              decorateLine(line, active ? fenceLineLast : fenceHiddenLineLast);
            } else {
              decorateLine(line, codeLine);
            }
          }

          const info = syntax.getChild("CodeInfo");
          const fenceText = doc.sliceString(first.from, first.to);
          const trimmedEnd = first.from + fenceText.replace(/\s+$/, "").length;
          const infoFrom = info ? info.from : trimmedEnd;
          const infoTo = info ? info.to : trimmedEnd;
          const language = info
            ? doc.sliceString(info.from, info.to).trim()
            : "";

          if (active) {
            styled.push(
              Decoration.widget({
                widget: new CodeLanguageWidget(language, infoFrom, infoTo),
                side: 1,
              }).range(first.to),
            );
            break;
          }

          // Hide the fences themselves rather than the whole lines: inside a
          // blockquote the leading `> ` belongs to a QuoteMark that is hidden
          // separately, and two overlapping replaces would not render.
          for (const mark of marksOf(syntax, "CodeMark")) {
            hide(mark.from, mark.to);
          }
          if (info) hide(info.from, info.to);
          break;
        }

        case "StrongEmphasis":
        case "Emphasis":
        case "Strikethrough": {
          if (isActive(node.from)) break;
          const markName =
            node.name === "Strikethrough"
              ? "StrikethroughMark"
              : "EmphasisMark";
          const marks = marksOf(syntax, markName);
          if (marks.length < 2) break;
          for (const mark of marks) hide(mark.from, mark.to);
          break;
        }

        case "InlineCode": {
          const marks = marksOf(syntax, "CodeMark");
          if (marks.length < 2) break;
          const open = marks[0];
          const close = marks[marks.length - 1];
          decorate(open.to, close.from, inlineCodeMark);
          if (isActive(node.from)) break;
          hide(open.from, open.to);
          hide(close.from, close.to);
          break;
        }

        case "Link": {
          const marks = marksOf(syntax, "LinkMark");
          if (marks.length < 3) break;
          if (isActive(node.from)) break;
          hide(marks[0].from, marks[0].to);
          hide(marks[1].from, node.to);
          break;
        }

        case "Image": {
          if (isActive(node.from)) break;
          const marks = marksOf(syntax, "LinkMark");
          const url = syntax.getChild("URL");
          if (marks.length < 3 || !url) break;

          const altFrom = marks[0].to;
          const altTo = marks[1].from;
          const alt = altTo > altFrom ? doc.sliceString(altFrom, altTo) : "";
          const line = doc.lineAt(node.from);
          const alone =
            doc.sliceString(line.from, node.from).trim() === "" &&
            doc.sliceString(node.to, line.to).trim() === "";

          if (alone) decorateLine(line, imageLine);
          replaceWith(
            Decoration.replace({
              widget: new ImageWidget(
                doc.sliceString(url.from, url.to),
                alt,
                node.from,
                node.to,
              ),
            }),
            node.from,
            node.to,
          );
          break;
        }

        default:
          break;
      }
    },
  });

  const replaced: Range<Decoration>[] = mergeHides(
    hides,
    widgets.map((widget) => widget.range),
  ).map((interval) => hidden.range(interval.from, interval.to));
  for (const widget of widgets) {
    replaced.push(widget.deco.range(widget.range.from, widget.range.to));
  }

  const atomic = Decoration.set(replaced, true);
  return {
    decorations: Decoration.set([...styled, ...replaced], true),
    atomic,
  };
}

/* ------------------------------------------------------------------ *
 * Extension
 * ------------------------------------------------------------------ */

/**
 * Renders the markdown as formatted text: syntax markers are hidden (except on
 * the lines holding the cursor), block constructs get their own styling, and
 * images, task checkboxes and the code-block language picker become widgets.
 */
export const livePreview = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    atomic: DecorationSet;

    constructor(view: EditorView) {
      const built = build(view);
      this.decorations = built.decorations;
      this.atomic = built.atomic;
    }

    update(update: ViewUpdate) {
      if (
        update.docChanged ||
        update.selectionSet ||
        update.viewportChanged ||
        update.focusChanged
      ) {
        const built = build(update.view);
        this.decorations = built.decorations;
        this.atomic = built.atomic;
      }
    }
  },
  {
    decorations: (plugin) => plugin.decorations,
    // Replace decorations are not atomic on their own, so hidden markers would
    // still swallow arrow-key movement without this.
    provide: (plugin) =>
      EditorView.atomicRanges.of(
        (view) => view.plugin(plugin)?.atomic ?? Decoration.none,
      ),
  },
);
