import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";

/** User profile page template: profile header plus paginated published posts. */
export function UserTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "%s - %s" .User.Username .Site.Name')}
        description={go(".User.Bio")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8">
          <div className="max-w-3xl mx-auto">
            {/* Profile header */}
            <div className="rounded-xl border bg-card text-card-foreground p-6 mb-8 flex items-center gap-4">
              {go("if .User.Avatar")}
              <img
                src={go(".User.Avatar")}
                alt={go(".User.Username")}
                className="w-16 h-16 rounded-full object-cover"
              />
              {go("end")}
              <div>
                <h1 className="text-2xl font-bold mb-1">
                  {go(".User.Username")}
                </h1>
                {go("if .User.Bio")}
                <p className="text-muted-foreground mb-1">{go(".User.Bio")}</p>
                {go("end")}
                <p className="text-sm text-muted-foreground">
                  Joined {go('date .User.CreatedAt "2006-01-02"')} ·{" "}
                  {go(".User.PostsCount")} posts
                </p>
              </div>
            </div>

            {/* Posts */}
            <div className="space-y-6">
              {go("if .Posts")}
              {go("range .Posts")}
              <article className="group rounded-xl border bg-card text-card-foreground p-6 shadow-sm hover:shadow-lg transition-shadow">
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
                  <span>{go('date .CreatedAt "2006-01-02"')}</span>
                  <span>·</span>
                  <span>{go(".CommentsCount")} comments</span>
                  <span>·</span>
                  <span>{go(".ViewCount")} views</span>
                </div>
              </article>
              {go("end")}
              {go("else")}
              <div className="rounded-xl border bg-card text-card-foreground p-12 text-center">
                <h3 className="text-lg font-semibold mb-2">No posts yet</h3>
                <p className="text-muted-foreground">
                  This user has not published anything.
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
          </div>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
