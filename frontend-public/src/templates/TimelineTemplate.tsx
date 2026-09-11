import { SiteFooter, SiteHeader, DocHead } from "../components/SiteLayout";
import { go } from "../lib/go";
import { ArrowLeft } from "../components/icons";

/**
 * Timeline page template (timeline.html). The page intro comes from
 * {{.Page.ContentHTML}}; the chronological post list is fetched client-side
 * from the public latest-posts API so themes stay free to style grouping.
 */
export function TimelineTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "%s - %s" .Page.Title .Site.Name')}
        description={go(".Page.Title")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8 max-w-4xl">
          <a
            href="/"
            className="inline-flex items-center gap-2 rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-3 mb-6 text-muted-foreground"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to home
          </a>
          <h1 className="text-3xl md:text-4xl font-bold mb-4">
            {go(".Page.Title")}
          </h1>
          <div className="mb-8">
            <div className="prose prose-lg max-w-none">
              {go(".Page.ContentHTML")}
            </div>
          </div>
          <div id="vexgo-timeline" className="vexgo-timeline">
            <p className="text-sm text-muted-foreground">Loading timeline…</p>
          </div>
        </main>
        <SiteFooter />
        <script>
          {
            "(function(){var el=document.getElementById('vexgo-timeline');if(!el)return;fetch('/api/stats/latest-posts?limit=200').then(function(r){return r.json()}).then(function(d){var posts=(d&&d.posts)||[];if(!posts.length){el.innerHTML='<p class=text-sm text-muted-foreground>No posts yet.</p>';return}var byYear={};posts.forEach(function(p){var y=(p.createdAt||'').slice(0,4)||'Unknown';(byYear[y]=byYear[y]||[]).push(p)});var years=Object.keys(byYear).sort().reverse();el.innerHTML=years.map(function(y){var items=byYear[y].map(function(p){var s=document.createElement('div');var a=document.createElement('a');a.href='/post/'+p.slug;a.textContent=p.title;a.className='vexgo-timeline-link';var t=document.createElement('span');t.textContent=(p.createdAt||'').slice(0,10);t.className='vexgo-timeline-date';s.className='vexgo-timeline-item';s.appendChild(t);s.appendChild(a);return s.outerHTML}).join('');return '<section class=vexgo-timeline-year><h2>'+y+'</h2>'+items+'</section>'}).join('')}).catch(function(){el.innerHTML='<p class=text-sm text-muted-foreground>Failed to load timeline.</p>'})})();"
          }
        </script>
      </body>
    </html>
  );
}
