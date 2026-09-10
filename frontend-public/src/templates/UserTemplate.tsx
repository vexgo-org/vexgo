import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";
import { ArrowLeft, Calendar, Heart, MessageCircle } from "../components/icons";

/** User profile page template: profile card plus paginated published posts. */
export function UserTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "%s - %s" .User.Username .Site.Name')}
        description={go(".User.Bio")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8 max-w-6xl">
          {/* Back button */}
          <a
            href="/"
            className="inline-flex items-center gap-2 rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-3 mb-6 text-muted-foreground"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to home
          </a>

          <div className="flex flex-col md:flex-row gap-6">
            {/* User info */}
            <div className="w-full md:w-64">
              <div className="rounded-xl border bg-card text-card-foreground p-6 sticky top-20 flex flex-col items-center text-center">
                {go("if .User.Avatar")}
                <img
                  src={go(".User.Avatar")}
                  alt={go(".User.Username")}
                  className="w-24 h-24 rounded-full object-cover mb-4"
                />
                {go("else")}
                <span className="w-24 h-24 rounded-full bg-primary/10 text-primary text-2xl font-medium flex items-center justify-center mb-4">
                  {go('printf "%.1s" .User.Username')}
                </span>
                {go("end")}
                <h2 className="text-xl font-bold mb-2">
                  {go(".User.Username")}
                </h2>
                {go("if .User.Bio")}
                <p className="text-sm text-muted-foreground mb-2">
                  {go(".User.Bio")}
                </p>
                {go("end")}
                <p className="text-sm text-muted-foreground mb-2">
                  Joined {go('date .User.CreatedAt "2006-01-02"')}
                </p>
                <div className="w-full border-t my-4" />
                <div className="w-full">
                  <p className="text-sm text-muted-foreground mb-2">
                    Total Posts
                  </p>
                  <p className="text-2xl font-bold">{go(".User.PostsCount")}</p>
                </div>
              </div>
            </div>

            {/* Post list */}
            <div className="flex-1">
              <h1 className="text-2xl font-bold mb-6">
                {go(".User.Username")}'s Posts
              </h1>

              {go("if .Posts")}
              <div className="space-y-4">
                {go("range .Posts")}
                <article className="rounded-xl border bg-card text-card-foreground p-6 shadow-sm hover:shadow-md transition-shadow">
                  <a href={go(".URL")} className="block group">
                    <h2 className="text-xl font-bold mb-3 group-hover:text-primary transition-colors">
                      {go(".Title")}
                    </h2>
                    {go("if .Excerpt")}
                    <p className="text-muted-foreground mb-4 line-clamp-2">
                      {go(".Excerpt")}
                    </p>
                    {go("end")}
                  </a>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
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
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span>{go(".AuthorName")}</span>
                        <span>·</span>
                        <span className="flex items-center gap-1">
                          <Calendar className="w-3 h-3" />
                          {go('date .CreatedAt "2006-01-02"')}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-4 text-sm text-muted-foreground">
                      {go("if .Category")}
                      <span className="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold">
                        {go(".Category")}
                      </span>
                      {go("end")}
                      <span className="flex items-center gap-1">
                        <MessageCircle className="w-3 h-3" />
                        {go(".CommentsCount")}
                      </span>
                      <span className="flex items-center gap-1">
                        <Heart className="w-3 h-3" />
                        {go(".LikesCount")}
                      </span>
                    </div>
                  </div>
                </article>
                {go("end")}
              </div>
              {go("else")}
              <div className="rounded-xl border bg-card text-card-foreground p-12 text-center">
                <h3 className="text-lg font-semibold mb-2">No posts yet</h3>
                <p className="text-muted-foreground">
                  This user has not published anything.
                </p>
              </div>
              {go("end")}

              {/* Pagination */}
              {go("if gt .Pagination.TotalPages 1")}
              <div className="flex items-center justify-between mt-6">
                <div className="text-sm text-muted-foreground">
                  Page {go(".Pagination.CurrentPage")} of{" "}
                  {go(".Pagination.TotalPages")}
                </div>
                <div className="flex gap-2">
                  {go("if .Pagination.HasPrev")}
                  <a
                    href={go(".Pagination.PrevURL")}
                    className="inline-flex items-center justify-center rounded-md border px-3 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                  >
                    ← Previous
                  </a>
                  {go("end")}
                  {go("if .Pagination.HasNext")}
                  <a
                    href={go(".Pagination.NextURL")}
                    className="inline-flex items-center justify-center rounded-md border px-3 h-9 text-sm font-medium hover:bg-secondary transition-colors"
                  >
                    Next →
                  </a>
                  {go("end")}
                </div>
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
