import { go } from "../lib/go";
import { Search } from "./icons";

/** SiteHeader is the top navigation bar shared by every public page. */
export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto px-4">
        <div className="flex h-16 items-center justify-between gap-4">
          {/* Logo */}
          <a href="/" className="flex items-center gap-2 shrink-0">
            {go("if .Site.Icon")}
            <img src={go(".Site.Icon")} alt="Logo" className="w-8 h-8" />
            {go("end")}
            <span className="text-xl font-bold hidden sm:inline">
              {go(".Site.Name")}
            </span>
          </a>

          {/* Search box - desktop */}
          <form
            action="/"
            method="get"
            className="hidden md:flex flex-1 max-w-md"
          >
            <div className="relative w-full">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type="search"
                name="search"
                placeholder="Search articles..."
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 pl-10 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              />
            </div>
          </form>

          {/* Nav links - desktop */}
          <nav className="hidden md:flex items-center gap-1">
            <a
              href="/"
              className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-4 py-2 text-muted-foreground"
            >
              Home
            </a>
          </nav>

          {/* Auth entry: Login/Register for guests, Profile when a session
              exists (the SPA stores its session in localStorage). */}
          <div id="vexgo-auth" className="flex items-center gap-2">
            <a
              href="/admin/login"
              className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-4 py-2 text-muted-foreground"
            >
              Login
            </a>
            <a
              href="/admin/register"
              className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors bg-primary text-primary-foreground shadow hover:bg-primary/90 h-9 px-4 py-2"
            >
              Register
            </a>
          </div>
        </div>
      </div>
      <script>
        {
          "(function(){var h=document.getElementById('vexgo-auth');if(!h||!localStorage.getItem('token'))return;var a=document.createElement('a');a.href='/admin/profile';a.className='inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors bg-primary text-primary-foreground shadow hover:bg-primary/90 h-9 px-4 py-2';a.textContent='Profile';h.textContent='';h.appendChild(a);})();"
        }
      </script>
    </header>
  );
}

/** SiteFooter is the footer shared by every public page. */
export function SiteFooter() {
  return (
    <footer className="border-t bg-muted/50">
      <div className="container mx-auto px-4 py-8">
        <div className="flex flex-col md:flex-row justify-between items-center gap-4">
          <div className="flex items-center gap-2">
            {go("if .Site.Icon")}
            <img src={go(".Site.Icon")} alt="Logo" className="w-6 h-6" />
            {go("end")}
            <span className="font-semibold">{go(".Site.Name")}</span>
          </div>
          <p className="text-sm text-muted-foreground">
            © 2026 {go(".Site.Name")}. All rights reserved.
          </p>
          <div className="flex gap-4">
            <a
              href="/"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              Home
            </a>
            <a
              href="/admin/login"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              Login
            </a>
          </div>
        </div>
      </div>
    </footer>
  );
}

/** DocHead renders the shared <head> metadata with per-page title/description. */
export function DocHead({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <head>
      <meta charSet="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>{title}</title>
      <meta name="description" content={description} />
      <link rel="stylesheet" href="/theme-assets/style.css" />
    </head>
  );
}
