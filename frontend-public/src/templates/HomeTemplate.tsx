import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
  Clock,
  Eye,
  Heart,
  MessageCircle,
  SearchX,
  Tag,
  TrendingUp,
} from "../components/icons";

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
          {/* Search hint */}
          {go("if .Query.Search")}
          <div className="mb-6 flex items-center gap-2">
            <SearchX className="w-5 h-5 text-muted-foreground" />
            <span className="text-muted-foreground">
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
                {/* Cover image */}
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
                  {/* Category and tags */}
                  <div className="flex flex-wrap items-center gap-2 mb-3">
                    {go("if .Category")}
                    <span className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground">
                      {go(".Category")}
                    </span>
                    {go("end")}
                    {go("range first .Tags 3")}
                    <span className="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold">
                      {go(".")}
                    </span>
                    {go("end")}
                  </div>

                  {/* Title */}
                  <h2 className="text-xl font-bold mb-3">
                    <a
                      href={go(".URL")}
                      className="hover:text-primary transition-colors line-clamp-2"
                    >
                      {go(".Title")}
                    </a>
                  </h2>

                  {/* Excerpt */}
                  <p className="text-muted-foreground mb-4 line-clamp-2">
                    {go(".Excerpt")}
                  </p>

                  {/* Author and stats */}
                  <div className="flex flex-col gap-2">
                    <a
                      href={go("userURL .AuthorID")}
                      className="flex items-center gap-3"
                    >
                      {go("if .AuthorAvatar")}
                      <img
                        src={go(".AuthorAvatar")}
                        alt="Avatar"
                        className="w-8 h-8 rounded-full object-cover"
                      />
                      {go("else")}
                      <span className="w-8 h-8 rounded-full bg-primary/10 text-primary text-sm font-medium flex items-center justify-center">
                        {go('printf "%.1s" .AuthorName')}
                      </span>
                      {go("end")}
                      <div className="flex items-center gap-2 text-sm">
                        <span className="text-muted-foreground hover:text-primary transition-colors">
                          {go(".AuthorName")}
                        </span>
                        <span className="text-muted-foreground">·</span>
                        <span className="flex items-center gap-1 text-muted-foreground">
                          <Calendar className="w-3 h-3" />
                          {go('date .CreatedAt "2006-01-02"')}
                        </span>
                      </div>
                    </a>
                    <div className="flex items-center gap-4 text-sm">
                      <span className="flex items-center gap-1 text-muted-foreground">
                        <Heart className="w-4 h-4" />
                        <span>{go(".LikesCount")}</span>
                      </span>
                      <span className="flex items-center gap-1 text-muted-foreground">
                        <MessageCircle className="w-4 h-4" />
                        <span>{go(".CommentsCount")}</span>
                      </span>
                      <span className="flex items-center gap-1 text-muted-foreground">
                        <Eye className="w-4 h-4" />
                        <span>{go(".ViewCount")}</span>
                      </span>
                    </div>
                  </div>
                </div>
              </article>
              {go("end")}
              {go("else")}
              <div className="rounded-xl border bg-card text-card-foreground p-12 text-center">
                <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mx-auto mb-4">
                  <SearchX className="w-8 h-8 text-muted-foreground" />
                </div>
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

              {/* Pagination */}
              {go("if gt .Pagination.TotalPages 1")}
              <nav className="flex items-center justify-center gap-1 pt-2">
                {go("if .Pagination.HasPrev")}
                <a
                  href={go(".Pagination.PrevURL")}
                  className="inline-flex items-center justify-center gap-1 rounded-md border px-3 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                >
                  <ChevronLeft className="w-4 h-4" />
                  Previous
                </a>
                {go("end")}
                {go("range .Pagination.Pages")}
                {go("if .Ellipsis")}
                <span className="px-2 text-sm text-muted-foreground">…</span>
                {go("else if .IsCurrent")}
                <span
                  aria-current="page"
                  className="inline-flex items-center justify-center rounded-md bg-primary px-3 h-9 text-sm font-medium text-primary-foreground"
                >
                  {go(".Number")}
                </span>
                {go("else")}
                <a
                  href={go(".URL")}
                  className="inline-flex items-center justify-center rounded-md border px-3 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                >
                  {go(".Number")}
                </a>
                {go("end")}
                {go("end")}
                {go("if .Pagination.HasNext")}
                <a
                  href={go(".Pagination.NextURL")}
                  className="inline-flex items-center justify-center gap-1 rounded-md border px-3 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                >
                  Next
                  <ChevronRight className="w-4 h-4" />
                </a>
                {go("end")}
              </nav>
              {go("end")}
            </div>

            {/* Sidebar */}
            <aside className="space-y-6">
              {/* Categories */}
              <div className="rounded-xl border bg-card text-card-foreground">
                <div className="p-6 pb-3 flex items-center gap-2">
                  <Tag className="w-4 h-4" />
                  <h3 className="text-lg font-semibold">Categories</h3>
                </div>
                <div className="p-6 pt-3">
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
              </div>

              {/* Popular tags */}
              <div className="rounded-xl border bg-card text-card-foreground">
                <div className="p-6 pb-3 flex items-center gap-2">
                  <Tag className="w-4 h-4" />
                  <h3 className="text-lg font-semibold">Popular Tags</h3>
                </div>
                <div className="p-6 pt-3">
                  {go("if .PopularTags")}
                  <div className="flex flex-wrap gap-2">
                    {go("range .PopularTags")}
                    <a
                      href={go("searchURL .Name")}
                      className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground hover:bg-accent transition-colors"
                    >
                      {go(".Name")}
                      <span className="ml-1 opacity-70">({go(".Count")})</span>
                    </a>
                    {go("end")}
                  </div>
                  {go("else")}
                  <p className="text-sm text-muted-foreground">
                    No popular tags yet.
                  </p>
                  {go("end")}
                </div>
              </div>

              {/* Popular posts */}
              <div className="rounded-xl border bg-card text-card-foreground">
                <div className="p-6 pb-3 flex items-center gap-2">
                  <TrendingUp className="w-4 h-4" />
                  <h3 className="text-lg font-semibold">Popular Posts</h3>
                </div>
                <div className="p-6 pt-3">
                  {go("if .PopularPosts")}
                  <div className="space-y-4">
                    {go("range $i, $p := .PopularPosts")}
                    <a
                      href={go("$p.URL")}
                      className="flex items-start gap-3 group"
                    >
                      <span className="text-lg font-bold text-muted-foreground w-6">
                        {go("add $i 1")}
                      </span>
                      <div>
                        <h4 className="text-sm font-medium line-clamp-2 group-hover:text-primary transition-colors">
                          {go("$p.Title")}
                        </h4>
                        <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                          <span className="flex items-center gap-1">
                            <Heart className="w-3 h-3" />
                            {go("$p.LikesCount")}
                          </span>
                          <span className="flex items-center gap-1">
                            <Eye className="w-3 h-3" />
                            {go("$p.ViewCount")}
                          </span>
                        </div>
                      </div>
                    </a>
                    {go("end")}
                  </div>
                  {go("else")}
                  <p className="text-sm text-muted-foreground">
                    No popular posts yet.
                  </p>
                  {go("end")}
                </div>
              </div>

              {/* About */}
              <div className="rounded-xl border bg-card text-card-foreground">
                <div className="p-6 pb-3 flex items-center gap-2">
                  <Clock className="w-4 h-4" />
                  <h3 className="text-lg font-semibold">About</h3>
                </div>
                <div className="p-6 pt-3">
                  {go("if .Site.Description")}
                  <p className="text-sm text-muted-foreground">
                    {go(".Site.Description")}
                  </p>
                  {go("else")}
                  <p className="text-sm text-muted-foreground">
                    A self-hosted blog CMS.
                  </p>
                  {go("end")}
                </div>
              </div>
            </aside>
          </div>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
