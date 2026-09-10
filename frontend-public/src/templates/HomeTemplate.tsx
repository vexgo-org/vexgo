import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";

/**
 * Home page template. Every `go(...)` expression becomes a Go template action
 * (`{{...}}`) in the built HTML, executed per request by the backend.
 */
export function HomeTemplate() {
  return (
    <html lang="en">
      <DocHead title={go(".Site.Name")} description={go(".Site.Description")} />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8">
          {go("if .Query.Search")}
          <div className="mb-6 flex items-center gap-2 text-sm text-muted-foreground">
            <span>
              Search results for <strong>{go(".Query.Search")}</strong>
            </span>
            <a
              href="/"
              className="underline hover:text-foreground transition-colors"
            >
              Clear
            </a>
          </div>
          {go("end")}

          <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
            {/* Main content area */}
            <div className="lg:col-span-3 space-y-6">
              {go("if .Posts")}
              {go("range .Posts")}
              <article className="group rounded-xl border bg-card text-card-foreground shadow-sm overflow-hidden hover:shadow-lg transition-shadow">
                {go("if .CoverImage")}
                <a href={go(".URL")} className="block">
                  <img
                    src={go(".CoverImage")}
                    alt={go(".Title")}
                    className="w-full h-auto object-contain group-hover:scale-105 transition-transform duration-300"
                  />
                </a>
                {go("end")}
                <div className="p-6">
                  <div className="flex flex-wrap items-center gap-2 mb-3">
                    {go("if .Category")}
                    <span className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground">
                      {go(".Category")}
                    </span>
                    {go("end")}
                    {go("range .Tags")}
                    <span className="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold">
                      {go(".")}
                    </span>
                    {go("end")}
                  </div>
                  <h2 className="text-xl font-bold mb-3">
                    <a
                      href={go(".URL")}
                      className="hover:text-primary transition-colors line-clamp-2"
                    >
                      {go(".Title")}
                    </a>
                  </h2>
                  <p className="text-muted-foreground mb-4 line-clamp-2">
                    {go(".Excerpt")}
                  </p>
                  <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                    <a
                      href={go("userURL .AuthorID")}
                      className="hover:text-primary transition-colors"
                    >
                      {go(".AuthorName")}
                    </a>
                    <span>·</span>
                    <span>{go('date .CreatedAt "2006-01-02"')}</span>
                    <span>·</span>
                    <span>{go(".CommentsCount")} comments</span>
                    <span>·</span>
                    <span>{go(".ViewCount")} views</span>
                  </div>
                </div>
              </article>
              {go("end")}
              {go("else")}
              <div className="rounded-xl border bg-card text-card-foreground p-12 text-center">
                <h3 className="text-lg font-semibold mb-2">No posts found</h3>
                <p className="text-muted-foreground">
                  {go("if .Query.Search")}
                  Try different keywords or clear the search.
                  {go("else")}
                  No posts have been published yet.
                  {go("end")}
                </p>
              </div>
              {go("end")}

              {go("if .Pagination.HasPrev")}
              <div className="flex justify-center gap-4 pt-2">
                <a
                  href={go(".Pagination.PrevURL")}
                  className="inline-flex items-center justify-center rounded-md border bg-card px-4 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                >
                  ← Previous
                </a>
                {go("end")}
                {go("if .Pagination.HasNext")}
                <a
                  href={go(".Pagination.NextURL")}
                  className="inline-flex items-center justify-center rounded-md border bg-card px-4 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                >
                  Next →
                </a>
              </div>
              {go("end")}
            </div>

            {/* Sidebar */}
            <aside className="space-y-6">
              <div className="rounded-xl border bg-card text-card-foreground p-6">
                <h3 className="text-lg font-semibold mb-3">Categories</h3>
                {go("if .Categories")}
                <div className="flex flex-wrap gap-2">
                  {go("range .Categories")}
                  <a
                    href={go("categoryURL .")}
                    className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground hover:bg-accent transition-colors"
                  >
                    {go(".")}
                  </a>
                  {go("end")}
                </div>
                {go("else")}
                <p className="text-sm text-muted-foreground">
                  No categories yet.
                </p>
                {go("end")}
              </div>

              {/* Hot posts */}
              <div className="rounded-xl border bg-card text-card-foreground p-6">
                <h3 className="text-lg font-semibold mb-3">Popular Posts</h3>
                {go("if .PopularPosts")}
                <ol className="space-y-3">
                  {go("range .PopularPosts")}
                  <li className="text-sm">
                    <a
                      href={go(".URL")}
                      className="font-medium hover:text-primary transition-colors line-clamp-2"
                    >
                      {go(".Title")}
                    </a>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      {go(".CommentsCount")} comments · {go(".ViewCount")} views
                    </div>
                  </li>
                  {go("end")}
                </ol>
                {go("else")}
                <p className="text-sm text-muted-foreground">
                  No popular posts yet.
                </p>
                {go("end")}
              </div>

              {/* Hot tags */}
              <div className="rounded-xl border bg-card text-card-foreground p-6">
                <h3 className="text-lg font-semibold mb-3">Popular Tags</h3>
                {go("if .PopularTags")}
                <div className="flex flex-wrap gap-2">
                  {go("range .PopularTags")}
                  <span className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground">
                    {go(".Name")}
                    <span className="ml-1 opacity-70">({go(".Count")})</span>
                  </span>
                  {go("end")}
                </div>
                {go("else")}
                <p className="text-sm text-muted-foreground">
                  No popular tags yet.
                </p>
                {go("end")}
              </div>
            </aside>
          </div>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
