import { useRef, useCallback, useEffect } from "react";
import { useIsDark } from "@/hooks/useIsDark";
import { uploadApi } from "@/lib/api";
import {
  MDXEditor,
  type MDXEditorMethods,
  toolbarPlugin,
  UndoRedo,
  BoldItalicUnderlineToggles,
  BlockTypeSelect,
  CreateLink,
  InsertImage,
  ListsToggle,
  CodeToggle,
  InsertThematicBreak,
  InsertTable,
  InsertCodeBlock,
  headingsPlugin,
  listsPlugin,
  linkPlugin,
  linkDialogPlugin,
  imagePlugin,
  thematicBreakPlugin,
  codeBlockPlugin,
  codeMirrorPlugin,
  directivesPlugin,
  frontmatterPlugin,
  AdmonitionDirectiveDescriptor,
  quotePlugin,
  markdownShortcutPlugin,
  diffSourcePlugin,
  DiffSourceToggleWrapper,
  ConditionalContents,
  ChangeCodeMirrorLanguage,
} from "@mdxeditor/editor";
import "@mdxeditor/editor/style.css";

interface RichTextEditorProps {
  content: string;
  onChange: (content: string) => void;
  placeholder?: string;
  originalContent?: string;
}

export function RichTextEditor({
  content,
  onChange,
  placeholder,
  originalContent,
}: RichTextEditorProps) {
  const editorRef = useRef<MDXEditorMethods>(null);
  const isDark = useIsDark();
  const isInternalChange = useRef(false);

  // Image upload handler
  const imageUploadHandler = useCallback(
    async (file: File): Promise<string> => {
      try {
        const response = await uploadApi.uploadFile(file);
        return response.data.file!.url;
      } catch (error) {
        console.error("Failed to upload image:", error);
        alert("Failed to upload image, please try again");
        throw error;
      }
    },
    [],
  );

  // Handle content changes
  const handleChange = useCallback(
    (markdown: string) => {
      isInternalChange.current = true;
      onChange(markdown);
    },
    [onChange],
  );

  // Use useEffect to sync content to editor
  // Only call setMarkdown when content is updated from external source (not from editor itself)
  // to avoid resetting cursor position on every input in source mode
  useEffect(() => {
    if (isInternalChange.current) {
      isInternalChange.current = false;
      return;
    }
    if (editorRef.current && content !== undefined) {
      editorRef.current.setMarkdown(content || "");
    }
  }, [content]);

  const defaultCodeBlockLanguage = "js";
  const codeBlockLanguages = {
    js: "JavaScript",
    jsx: "JavaScript (React)",
    ts: "TypeScript",
    tsx: "TypeScript (React)",
    css: "CSS",
    html: "HTML",
    python: "Python",
    bash: "Bash",
    shell: "Shell",
    json: "JSON",
    yaml: "YAML",
    markdown: "Markdown",
  };

  return (
    <div className="border rounded-lg overflow-hidden bg-background">
      <MDXEditor
        ref={editorRef}
        markdown={content || ""}
        onChange={handleChange}
        placeholder={placeholder || "Start writing..."}
        contentEditableClassName="prose"
        className={isDark ? "dark-theme dark-editor" : "dark-editor"}
        plugins={[
          headingsPlugin(),
          listsPlugin(),
          linkPlugin(),
          linkDialogPlugin(),
          imagePlugin({
            imageUploadHandler,
          }),
          thematicBreakPlugin(),
          codeBlockPlugin({ defaultCodeBlockLanguage }),
          codeMirrorPlugin({ codeBlockLanguages }),
          directivesPlugin({
            directiveDescriptors: [AdmonitionDirectiveDescriptor],
          }),
          frontmatterPlugin(),
          quotePlugin(),
          markdownShortcutPlugin(),
          diffSourcePlugin({
            viewMode: "rich-text",
            diffMarkdown: originalContent ?? "",
            readOnlyDiff: false,
          }),
          toolbarPlugin({
            toolbarClassName:
              "flex flex-wrap items-center gap-1 p-2 border-b bg-muted/50",
            toolbarContents: () => (
              <DiffSourceToggleWrapper>
                <ConditionalContents
                  options={[
                    {
                      when: (editor) => editor?.editorType === "codeblock",
                      contents: () => <ChangeCodeMirrorLanguage />,
                    },
                    {
                      fallback: () => (
                        <>
                          <UndoRedo />
                          <span className="w-px h-6 bg-border mx-1" />
                          <BoldItalicUnderlineToggles />
                          <CodeToggle />
                          <span className="w-px h-6 bg-border mx-1" />
                          <BlockTypeSelect />
                          <span className="w-px h-6 bg-border mx-1" />
                          <ListsToggle />
                          <span className="w-px h-6 bg-border mx-1" />
                          <CreateLink />
                          <InsertImage />
                          <InsertTable />
                          <InsertCodeBlock />
                          <InsertThematicBreak />
                        </>
                      ),
                    },
                  ]}
                />
              </DiffSourceToggleWrapper>
            ),
          }),
        ]}
      />
    </div>
  );
}
