import { EditorView, keymap } from "@codemirror/view";

import { editorCallbacks } from "./config";
import {
  toggleBold,
  toggleBulletList,
  toggleInlineCode,
  toggleItalic,
  toggleOrderedList,
} from "../commands";

/** Markers that pair themselves while typing. */
const PAIR_CHARS = ["*", "_", "`"];

function pairOrSkip(
  view: EditorView,
  from: number,
  to: number,
  text: string,
): boolean {
  const { state } = view;

  // Typing the second half of a pair that is already there steps over it
  // instead of doubling up, so `**` stays `**`.
  if (state.sliceDoc(to, to + text.length) === text) {
    view.dispatch({ selection: { anchor: to + text.length } });
    return true;
  }

  if (to > from) {
    const selected = state.sliceDoc(from, to);
    view.dispatch({
      changes: { from, to, insert: text + selected + text },
      selection: { anchor: from + text.length, head: to + text.length },
    });
    return true;
  }

  const line = state.doc.lineAt(from);
  const before = state.sliceDoc(line.from, from);
  // At the start of a line these characters are almost always a list marker,
  // not the opening half of an emphasis pair.
  if (before.trim() === "") return false;
  if (text === "_" && /\w$/.test(before)) return false;

  view.dispatch({
    changes: { from, to, insert: text + text },
    selection: { anchor: from + text.length },
  });
  return true;
}

export const markdownInputHandler = EditorView.inputHandler.of(
  (view, from, to, text) => {
    if (!PAIR_CHARS.includes(text)) return false;
    return pairOrSkip(view, from, to, text);
  },
);

export const markdownKeymap = keymap.of([
  { key: "Mod-b", run: toggleBold, preventDefault: true },
  { key: "Mod-i", run: toggleItalic, preventDefault: true },
  { key: "Mod-e", run: toggleInlineCode, preventDefault: true },
  {
    key: "Mod-Shift-7",
    preventDefault: true,
    run: (view) => {
      toggleOrderedList(view);
      return true;
    },
  },
  {
    key: "Mod-Shift-8",
    preventDefault: true,
    run: (view) => {
      toggleBulletList(view);
      return true;
    },
  },
  {
    key: "Mod-k",
    preventDefault: true,
    run: (view) => {
      view.state.facet(editorCallbacks).onRequestLink?.();
      return true;
    },
  },
]);
