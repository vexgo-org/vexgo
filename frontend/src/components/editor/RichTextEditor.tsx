import { useRef, useCallback, useEffect } from "react";
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
  InsertAdmonition,
  InsertFrontmatter,
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
}

export function RichTextEditor({
  content,
  onChange,
  placeholder,
}: RichTextEditorProps) {
  const editorRef = useRef<MDXEditorMethods>(null);
  const isInternalChange = useRef(false);

  // 图片上传处理函数
  const imageUploadHandler = useCallback(
    async (file: File): Promise<string> => {
      try {
        const response = await uploadApi.uploadFile(file);
        return response.data.file!.url;
      } catch (error) {
        console.error("上传图片失败:", error);
        alert("上传图片失败，请重试");
        throw error;
      }
    },
    [],
  );

  // 处理内容变化
  const handleChange = useCallback(
    (markdown: string) => {
      isInternalChange.current = true;
      onChange(markdown);
    },
    [onChange],
  );

  // 使用 useEffect 来同步内容到编辑器
  // 仅在内容由外部（非编辑器本身）更新时才调用 setMarkdown，
  // 避免源码模式下每次输入都重置光标位置
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
        placeholder={placeholder || "开始写作..."}
        contentEditableClassName="prose"
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
                          <InsertAdmonition />
                          <InsertFrontmatter />
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
