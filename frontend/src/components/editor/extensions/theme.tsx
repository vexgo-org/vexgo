import { EditorView } from "@codemirror/view";
import { HighlightStyle } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

const mono =
  'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace';

/**
 * Colours come from the app's design tokens, so a single stylesheet serves
 * both themes. The `dark` flag only tells CodeMirror which of its own
 * built-in defaults (`&light` / `&dark`) to apply.
 */
function themeSpec() {
  return {
    "&": {
      backgroundColor: "hsl(var(--background))",
      color: "hsl(var(--foreground))",
    },
    "&.cm-focused": {
      outline: "none",
    },
    ".cm-scroller": {
      overflow: "auto",
      maxHeight: "min(70vh, 820px)",
    },
    ".cm-content": {
      padding: "16px 20px",
      minHeight: "var(--vexgo-md-min-height, 480px)",
      caretColor: "hsl(var(--foreground))",
    },
    // Active-line highlighting draws an ugly band under tall rich-text blocks,
    // so it is switched off in favour of the caret alone.
    ".cm-activeLine": {
      backgroundColor: "transparent",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "transparent",
    },
    ".cm-cursor, .cm-dropCursor": {
      borderLeftColor: "hsl(var(--foreground))",
      borderLeftWidth: "2px",
    },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
      {
        backgroundColor: "hsl(var(--primary) / 0.18)",
      },
    ".cm-selectionMatch": {
      backgroundColor: "hsl(var(--primary) / 0.12)",
    },
    ".cm-matchingBracket, .cm-nonmatchingBracket": {
      backgroundColor: "hsl(var(--primary) / 0.16)",
      color: "inherit",
    },
    ".cm-gutters": {
      backgroundColor: "hsl(var(--muted) / 0.35)",
      color: "hsl(var(--muted-foreground))",
      border: "none",
      borderRight: "1px solid hsl(var(--border))",
      paddingRight: "4px",
    },
    ".cm-lineNumbers .cm-gutterElement": {
      padding: "0 8px 0 12px",
      minWidth: "32px",
    },
    ".cm-placeholder": {
      color: "hsl(var(--muted-foreground))",
    },
    ".cm-tooltip": {
      backgroundColor: "hsl(var(--popover))",
      color: "hsl(var(--popover-foreground))",
      border: "1px solid hsl(var(--border))",
      borderRadius: "6px",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "hsl(var(--accent))",
      color: "hsl(var(--accent-foreground))",
    },
    ".cm-panels": {
      backgroundColor: "hsl(var(--muted))",
      color: "hsl(var(--foreground))",
    },

    /* ---------- rich-text block styling ---------- */
    // Headings get their type scale from the highlight style; the line
    // decoration only contributes spacing, so the two never compound.
    ".cm-md-h1": { paddingTop: "0.6em", paddingBottom: "0.15em" },
    ".cm-md-h2": { paddingTop: "0.6em", paddingBottom: "0.15em" },
    ".cm-md-h3": { paddingTop: "0.5em", paddingBottom: "0.1em" },
    ".cm-md-h4, .cm-md-h5, .cm-md-h6": { paddingTop: "0.4em" },
    ".cm-md-quote": {
      borderLeft: "3px solid hsl(var(--border))",
      paddingLeft: "14px",
      marginLeft: "-3px",
    },
    // `@lezer/markdown` tags only `InlineCode CodeText` as monospace, so a
    // fenced block whose body is parsed by a nested language never receives a
    // monospace span — it would inherit the UI sans font. Source mode gets its
    // monospace from the scroller rule, so the block has to carry it here for
    // rich mode to look like code.
    ".cm-md-code-line": {
      backgroundColor: "hsl(var(--muted) / 0.55)",
      fontFamily: mono,
    },
    ".cm-md-code-fence": {
      backgroundColor: "hsl(var(--muted) / 0.55)",
      color: "hsl(var(--muted-foreground))",
      borderRadius: "6px 6px 0 0",
      fontFamily: mono,
    },
    ".cm-md-code-fence-last": {
      borderRadius: "0 0 6px 6px",
    },
    // When the cursor is outside the block the fence lines carry no text, so
    // collapsing their line box makes the block read as a single surface.
    ".cm-md-code-fence-hidden": {
      backgroundColor: "hsl(var(--muted) / 0.55)",
      fontSize: "0",
      lineHeight: "0",
      paddingTop: "6px",
      borderRadius: "6px 6px 0 0",
    },
    ".cm-md-code-fence-hidden.cm-md-code-fence-last": {
      paddingTop: "0",
      paddingBottom: "6px",
      borderRadius: "0 0 6px 6px",
    },
    // The horizontal rule hides its own `---` text and paints a border
    // instead, which avoids a block widget in the document flow.
    ".cm-md-hr": {
      fontSize: "0",
      lineHeight: "0",
      paddingTop: "20px",
      paddingBottom: "12px",
      borderBottom: "1px solid hsl(var(--border))",
    },
    // A collapsed line whose text is hidden entirely (setext underlines).
    ".cm-md-collapsed": {
      fontSize: "0",
      lineHeight: "0",
    },
    ".cm-md-image-line": {
      textAlign: "center",
    },
    ".cm-md-inline-code": {
      backgroundColor: "hsl(var(--muted))",
      borderRadius: "4px",
      padding: "1px 4px",
    },
    ".cm-md-list-mark": {
      color: "hsl(var(--muted-foreground))",
    },
  };
}

export function createEditorTheme(dark: boolean) {
  return EditorView.theme(themeSpec(), { dark });
}

/**
 * Markdown typography plus the syntax colours used by both modes: rich mode
 * hides the markers but keeps the styling, source mode shows everything.
 */
export const markdownHighlight = HighlightStyle.define([
  { tag: t.heading1, fontSize: "1.7em", fontWeight: "700", lineHeight: "1.35" },
  { tag: t.heading2, fontSize: "1.4em", fontWeight: "700", lineHeight: "1.35" },
  { tag: t.heading3, fontSize: "1.18em", fontWeight: "600", lineHeight: "1.4" },
  { tag: t.heading4, fontSize: "1.05em", fontWeight: "600" },
  { tag: t.heading5, fontSize: "1em", fontWeight: "600" },
  {
    tag: t.heading6,
    fontSize: "0.95em",
    fontWeight: "600",
    color: "hsl(var(--muted-foreground))",
  },
  { tag: t.strong, fontWeight: "700" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strikethrough, textDecoration: "line-through" },
  { tag: t.monospace, fontFamily: mono },
  // The theme's `--primary` is near-identical to the body colour, so links
  // also need an underline to stay recognisable in rich mode.
  {
    tag: t.link,
    color: "hsl(var(--primary))",
    textDecoration: "underline",
    textUnderlineOffset: "2px",
    textDecorationThickness: "1px",
  },
  { tag: t.url, color: "hsl(var(--primary))" },
  { tag: t.quote, color: "hsl(var(--muted-foreground))" },
  { tag: t.list, color: "hsl(var(--foreground))" },
  { tag: t.contentSeparator, color: "hsl(var(--border))" },
  {
    tag: t.processingInstruction,
    color: "hsl(var(--muted-foreground))",
    opacity: "0.65",
  },
  { tag: t.labelName, color: "hsl(var(--muted-foreground))" },
  { tag: t.escape, color: "hsl(var(--muted-foreground))" },
  {
    tag: t.comment,
    color: "hsl(var(--muted-foreground))",
    fontStyle: "italic",
  },

  /* fenced-code languages */
  { tag: [t.keyword, t.modifier, t.controlKeyword], color: "hsl(265 70% 62%)" },
  { tag: [t.string, t.special(t.string)], color: "hsl(150 55% 42%)" },
  { tag: [t.number, t.bool, t.null, t.atom], color: "hsl(25 80% 52%)" },
  { tag: [t.typeName, t.className, t.namespace], color: "hsl(200 70% 48%)" },
  { tag: [t.propertyName, t.attributeName], color: "hsl(200 60% 45%)" },
  { tag: [t.function(t.variableName), t.labelName], color: "hsl(220 70% 55%)" },
  {
    tag: [t.definition(t.variableName), t.variableName],
    color: "hsl(var(--foreground))",
  },
  { tag: [t.tagName, t.angleBracket], color: "hsl(350 65% 55%)" },
  { tag: t.operator, color: "hsl(210 20% 45%)" },
  { tag: t.punctuation, color: "hsl(210 15% 45%)" },
  { tag: t.bracket, color: "hsl(210 15% 45%)" },
  { tag: t.regexp, color: "hsl(320 60% 50%)" },
  { tag: t.meta, color: "hsl(var(--muted-foreground))" },
  { tag: t.invalid, color: "hsl(var(--destructive))" },
]);
