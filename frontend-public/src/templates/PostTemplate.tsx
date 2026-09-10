import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";

/** Post detail page template ({{.Post.ContentHTML}} is pre-rendered markdown). */
export function PostTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "%s - %s" .Post.Title .Site.Name')}
        description={go(".Post.Excerpt")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8">
          <article className="max-w-3xl mx-auto">
            {go("if .Post.CoverImage")}
            <img
              src={go(".Post.CoverImage")}
              alt={go(".Post.Title")}
              className="w-full h-auto object-contain rounded-xl mb-6"
            />
            {go("end")}

            <h1 className="text-3xl font-bold mb-4">{go(".Post.Title")}</h1>

            <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground mb-4">
              <a
                href={go("userURL .Post.AuthorID")}
                className="hover:text-primary transition-colors"
              >
                {go(".Post.AuthorName")}
              </a>
              <span>·</span>
              <span>{go('date .Post.CreatedAt "2006-01-02"')}</span>
              <span>·</span>
              <span>{go(".Post.ViewCount")} views</span>
              <span>·</span>
              <span>{go(".Post.CommentsCount")} comments</span>
            </div>

            {go("if .Post.Category")}
            <div className="flex flex-wrap items-center gap-2 mb-8">
              <span className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground">
                {go(".Post.Category")}
              </span>
              {go("range .Post.Tags")}
              <span className="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold">
                {go(".")}
              </span>
              {go("end")}
            </div>
            {go("end")}

            <div className="prose prose-lg max-w-none">
              {go(".Post.ContentHTML")}
            </div>

            {/* Comment section: rendered by the built-in widget (see
                widget/comments.js). The widget reads the post id from this
                container and styles itself with inline styles, so any theme
                can adopt it with the same two lines. */}
            <div id="vexgo-comments" data-post-id={go(".Post.ID")}></div>
          </article>
        </main>
        <SiteFooter /> <script src="/theme-assets/comments.js" defer></script>
      </body>
    </html>
  );
}
