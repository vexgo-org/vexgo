import { EditorView } from "@codemirror/view";

import { editorCallbacks } from "./config";

function imageFiles(files: FileList | null | undefined): File[] {
  if (!files) return [];
  return Array.from(files).filter((file) => file.type.startsWith("image/"));
}

/** Places `![alt](url)` on a line of its own so it renders as a block image. */
export function insertImageMarkdown(
  view: EditorView,
  entries: { url: string; alt: string }[],
) {
  if (entries.length === 0) return;
  const { state } = view;
  const range = state.selection.main;
  const before =
    range.from > 0 ? state.sliceDoc(range.from - 1, range.from) : "\n";

  const body = entries.map(({ url, alt }) => `![${alt}](${url})`).join("\n");
  // The cursor lands below the images so they render straight away instead of
  // sitting on an "active" line that shows raw source.
  const insert = `${before === "\n" ? "" : "\n"}${body}\n`;

  view.dispatch({
    changes: { from: range.from, to: range.to, insert },
    selection: { anchor: range.from + insert.length },
    scrollIntoView: true,
  });
}

async function uploadAndInsert(view: EditorView, files: File[]) {
  if (view.state.readOnly) return;
  const { uploadImage } = view.state.facet(editorCallbacks);
  if (!uploadImage) return;
  const results = await Promise.all(
    files.map(async (file) => {
      try {
        const url = await uploadImage(file);
        return url ? { url, alt: file.name.replace(/\.[^.]+$/, "") } : null;
      } catch {
        // The upload callback already reported the failure to the user.
        return null;
      }
    }),
  );
  const entries = results.filter(
    (entry): entry is { url: string; alt: string } => entry !== null,
  );
  if (entries.length > 0) insertImageMarkdown(view, entries);
}

/**
 * Pasting or dropping an image file uploads it and inserts the markdown,
 * matching what a writer expects from a rich-text surface.
 */
export const imageInteractions = EditorView.domEventHandlers({
  paste(event, view) {
    const files = imageFiles(event.clipboardData?.files);
    if (files.length === 0) return false;
    event.preventDefault();
    void uploadAndInsert(view, files);
    return true;
  },
  drop(event, view) {
    const files = imageFiles(event.dataTransfer?.files);
    if (files.length === 0) return false;
    event.preventDefault();
    const at = view.posAtCoords({ x: event.clientX, y: event.clientY });
    if (at !== null) {
      view.dispatch({ selection: { anchor: at } });
    }
    void uploadAndInsert(view, files);
    return true;
  },
  dragover(event) {
    if (imageFiles(event.dataTransfer?.files).length === 0) return false;
    event.preventDefault();
    return true;
  },
});
