import { useCallback, useEffect, useRef, useState } from "react";
import { Compartment, EditorState, type StateEffect } from "@codemirror/state";
import {
  EditorView,
  crosshairCursor,
  drawSelection,
  dropCursor,
  highlightSpecialChars,
  keymap,
  lineNumbers,
  placeholder as placeholderExtension,
  rectangularSelection,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  bracketMatching,
  indentOnInput,
  syntaxHighlighting,
  syntaxTree,
} from "@codemirror/language";
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
} from "@codemirror/autocomplete";
import { highlightSelectionMatches, searchKeymap } from "@codemirror/search";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import type { SyntaxNode } from "@lezer/common";

import type { EditorSnapshot, EditorMode } from "./types";
import { editorCallbacks, type EditorCallbacks } from "./extensions/config";
import { createEditorTheme, markdownHighlight } from "./extensions/theme";
import { livePreview } from "./extensions/livePreview";
import { tableWidgets } from "./extensions/tables";
import { imageInteractions } from "./extensions/interactions";
import { markdownInputHandler, markdownKeymap } from "./extensions/keymap";
import { canRedo, canUndo } from "./commands";

const EMPTY_SNAPSHOT: EditorSnapshot = {
  bold: false,
  italic: false,
  strike: false,
  inlineCode: false,
  blockType: "paragraph",
  inBulletList: false,
  inOrderedList: false,
  inTaskList: false,
  inQuote: false,
  inCodeBlock: false,
  codeLanguage: "",
  codeInfoRange: null,
  canUndo: false,
  canRedo: false,
};

const HEADING_NAMES = [
  "ATXHeading1",
  "ATXHeading2",
  "ATXHeading3",
  "ATXHeading4",
  "ATXHeading5",
  "ATXHeading6",
  "SetextHeading1",
  "SetextHeading2",
];

function enclosing(
  state: EditorState,
  pos: number,
  names: readonly string[],
): SyntaxNode | null {
  for (const at of pos > 0 ? [pos, pos - 1] : [pos]) {
    let node: SyntaxNode | null = syntaxTree(state).resolveInner(at, -1);
    while (node) {
      if (names.includes(node.name)) return node;
      node = node.parent;
    }
  }
  return null;
}

function hasEnclosing(
  state: EditorState,
  pos: number,
  names: readonly string[],
): boolean {
  return enclosing(state, pos, names) !== null;
}

function computeSnapshot(state: EditorState): EditorSnapshot {
  const pos = state.selection.main.from;
  const code = enclosing(state, pos, ["FencedCode"]);
  const heading = enclosing(state, pos, HEADING_NAMES);

  let blockType: EditorSnapshot["blockType"] = "paragraph";
  if (code) blockType = "code";
  else if (heading) {
    const level = Number(heading.name.slice(-1));
    blockType = `heading${Math.min(level, 4)}` as EditorSnapshot["blockType"];
  } else if (hasEnclosing(state, pos, ["Blockquote"])) blockType = "quote";

  let codeLanguage = "";
  let codeInfoRange: EditorSnapshot["codeInfoRange"] = null;
  if (code) {
    const info = code.getChild("CodeInfo");
    if (info) {
      codeLanguage = state.sliceDoc(info.from, info.to).trim();
      codeInfoRange = { from: info.from, to: info.to };
    } else {
      const line = state.doc.lineAt(code.from);
      const trimmed =
        line.from +
        state.sliceDoc(line.from, line.to).replace(/\s+$/, "").length;
      codeInfoRange = { from: trimmed, to: trimmed };
    }
  }

  return {
    bold: hasEnclosing(state, pos, ["StrongEmphasis"]),
    italic: hasEnclosing(state, pos, ["Emphasis"]),
    strike: hasEnclosing(state, pos, ["Strikethrough"]),
    inlineCode: hasEnclosing(state, pos, ["InlineCode"]),
    blockType,
    inBulletList: hasEnclosing(state, pos, ["BulletList"]),
    inOrderedList: hasEnclosing(state, pos, ["OrderedList"]),
    inTaskList: hasEnclosing(state, pos, ["Task"]),
    inQuote: hasEnclosing(state, pos, ["Blockquote"]),
    inCodeBlock: code !== null,
    codeLanguage,
    codeInfoRange,
    canUndo: canUndo(state),
    canRedo: canRedo(state),
  };
}

function sameSnapshot(a: EditorSnapshot, b: EditorSnapshot): boolean {
  return (
    a.bold === b.bold &&
    a.italic === b.italic &&
    a.strike === b.strike &&
    a.inlineCode === b.inlineCode &&
    a.blockType === b.blockType &&
    a.inBulletList === b.inBulletList &&
    a.inOrderedList === b.inOrderedList &&
    a.inTaskList === b.inTaskList &&
    a.inQuote === b.inQuote &&
    a.inCodeBlock === b.inCodeBlock &&
    a.codeLanguage === b.codeLanguage &&
    a.codeInfoRange?.from === b.codeInfoRange?.from &&
    a.codeInfoRange?.to === b.codeInfoRange?.to &&
    a.canUndo === b.canUndo &&
    a.canRedo === b.canRedo
  );
}

/**
 * Reconfiguring through a plain function keeps the `.current` access out of
 * the hook callbacks, where it would otherwise be reported as a dependency.
 */
function dispatchEffects(
  ref: React.RefObject<EditorView | null>,
  effects: StateEffect<unknown>,
) {
  ref.current?.dispatch({ effects });
}

function focusEditor(ref: React.RefObject<EditorView | null>) {
  ref.current?.focus();
}

/** Normalises an external value to the `\n` separators CodeMirror uses. */
function toEditorText(text: string, eol: string) {
  return eol === "\n" ? text : text.replace(/\r\n/g, "\n");
}

/** Restores the document's original line endings on the way out. */
function fromEditorText(text: string, eol: string) {
  return eol === "\n" ? text : text.replace(/\n/g, eol);
}

export interface UseCodeMirrorOptions {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  readOnly?: boolean;
  mode: EditorMode;
  dark: boolean;
  callbacks: EditorCallbacks;
  /**
   * Lets the caller hold the editor view before the hook runs, which keeps
   * callbacks that need the view free of forward references.
   */
  viewRef?: React.RefObject<EditorView | null>;
}

export interface UseCodeMirrorResult {
  containerRef: React.RefObject<HTMLDivElement | null>;
  viewRef: React.RefObject<EditorView | null>;
  snapshot: EditorSnapshot;
  focus: () => void;
}

export function useCodeMirror({
  value,
  onChange,
  placeholder,
  readOnly = false,
  mode,
  dark,
  callbacks,
  viewRef: externalViewRef,
}: UseCodeMirrorOptions): UseCodeMirrorResult {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const internalViewRef = useRef<EditorView | null>(null);
  const viewRef = externalViewRef ?? internalViewRef;
  const [snapshot, setSnapshot] = useState<EditorSnapshot>(EMPTY_SNAPSHOT);

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  /**
   * CodeMirror stores the document with `\n` separators, so a CRLF document
   * would silently lose its line endings on the first edit. Remember which
   * ending the document arrived with and restore it on the way out, keeping
   * the "save exactly what was typed" promise.
   */
  const eolRef = useRef(value.includes("\r\n") ? "\r\n" : "\n");

  // Read once when the view is created; later changes flow through the
  // compartments below, so the mount effect never needs to re-run.
  const initial = useRef({
    value,
    dark,
    readOnly,
    placeholder,
    mode,
    callbacks,
  });
  const themeCompartment = useRef(new Compartment()).current;
  const modeCompartment = useRef(new Compartment()).current;
  const readOnlyCompartment = useRef(new Compartment()).current;
  const placeholderCompartment = useRef(new Compartment()).current;
  const callbacksCompartment = useRef(new Compartment()).current;

  const modeExtensions = useCallback(
    (next: EditorMode) =>
      next === "rich" ? [livePreview, tableWidgets] : [lineNumbers()],
    [],
  );

  const placeholderExtensions = useCallback(
    (text: string | undefined) => (text ? [placeholderExtension(text)] : []),
    [],
  );

  useEffect(() => {
    const parent = containerRef.current;
    if (!parent) return;
    const start = initial.current;

    const view = new EditorView({
      parent,
      state: EditorState.create({
        doc: start.value,
        extensions: [
          themeCompartment.of(createEditorTheme(start.dark)),
          callbacksCompartment.of(editorCallbacks.of(start.callbacks)),
          markdown({ base: markdownLanguage, codeLanguages: languages }),
          syntaxHighlighting(markdownHighlight),
          history(),
          drawSelection(),
          dropCursor(),
          highlightSpecialChars(),
          rectangularSelection(),
          crosshairCursor(),
          bracketMatching(),
          closeBrackets(),
          autocompletion(),
          indentOnInput(),
          highlightSelectionMatches(),
          EditorView.lineWrapping,
          imageInteractions,
          markdownInputHandler,
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...searchKeymap,
            ...historyKeymap,
            ...completionKeymap,
          ]),
          markdownKeymap,
          readOnlyCompartment.of([
            EditorState.readOnly.of(start.readOnly),
            EditorView.editable.of(!start.readOnly),
          ]),
          placeholderCompartment.of(placeholderExtensions(start.placeholder)),
          modeCompartment.of(modeExtensions(start.mode)),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(
                fromEditorText(update.state.doc.toString(), eolRef.current),
              );
            }
            if (
              update.docChanged ||
              update.selectionSet ||
              update.focusChanged
            ) {
              const next = computeSnapshot(update.state);
              setSnapshot((prev) => (sameSnapshot(prev, next) ? prev : next));
            }
          }),
        ],
      }),
    });

    viewRef.current = view;
    setSnapshot(computeSnapshot(view.state));

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [
    modeExtensions,
    placeholderExtensions,
    themeCompartment,
    modeCompartment,
    readOnlyCompartment,
    placeholderCompartment,
    callbacksCompartment,
    viewRef,
  ]);

  /* External value updates (loading a post, resetting a form) replace the
     document; editor-originated updates are already in sync and skipped. */
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    eolRef.current = value.includes("\r\n") ? "\r\n" : "\n";
    const next = toEditorText(value, eolRef.current);
    if (view.state.doc.toString() === next) return;
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: next },
      selection: { anchor: 0 },
      scrollIntoView: true,
    });
  }, [value, viewRef]);

  useEffect(() => {
    dispatchEffects(
      viewRef,
      themeCompartment.reconfigure(createEditorTheme(dark)),
    );
  }, [dark, themeCompartment, viewRef]);

  useEffect(() => {
    dispatchEffects(viewRef, modeCompartment.reconfigure(modeExtensions(mode)));
  }, [mode, modeExtensions, modeCompartment, viewRef]);

  useEffect(() => {
    dispatchEffects(
      viewRef,
      readOnlyCompartment.reconfigure([
        EditorState.readOnly.of(readOnly),
        EditorView.editable.of(!readOnly),
      ]),
    );
  }, [readOnly, readOnlyCompartment, viewRef]);

  useEffect(() => {
    dispatchEffects(
      viewRef,
      placeholderCompartment.reconfigure(placeholderExtensions(placeholder)),
    );
  }, [placeholder, placeholderExtensions, placeholderCompartment, viewRef]);

  useEffect(() => {
    dispatchEffects(
      viewRef,
      callbacksCompartment.reconfigure(editorCallbacks.of(callbacks)),
    );
  }, [callbacks, callbacksCompartment, viewRef]);

  const focus = useCallback(() => focusEditor(viewRef), [viewRef]);

  return { containerRef, viewRef, snapshot, focus };
}
