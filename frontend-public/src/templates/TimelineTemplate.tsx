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
            <p className="vexgo-timeline-empty">Loading timeline…</p>
          </div>
        </main>
        <SiteFooter />
        <script>
          {
            "(function(){var el=document.getElementById('vexgo-timeline');if(!el)return;function esc(s){var d=document.createElement('div');d.textContent=(s==null?'':String(s));return d.innerHTML}fetch('/api/stats/latest-posts?limit=200').then(function(r){return r.json()}).then(function(d){var posts=(d&&d.posts)||[];if(!posts.length){el.innerHTML='<p class=\"vexgo-timeline-empty\">No posts yet.</p>';return}var byYear={};posts.forEach(function(p){var y=(p.createdAt||'').slice(0,4)||'Unknown';(byYear[y]=byYear[y]||[]).push(p)});var years=Object.keys(byYear).sort().reverse();var html='<p class=\"vexgo-timeline-total\">'+posts.length+(posts.length===1?' post':' posts')+' in total</p>';years.forEach(function(y){var items=byYear[y];html+='<section class=\"vexgo-timeline-year\"><div class=\"vexgo-timeline-year-badge\"><span class=\"vexgo-timeline-year-name\">'+esc(y)+'</span><span class=\"vexgo-timeline-year-count\">'+items.length+'</span></div><ol class=\"vexgo-timeline-list\">';items.forEach(function(p,i){html+='<li class=\"vexgo-timeline-item\" style=\"animation-delay:'+Math.min(i*50,500)+'ms\"><div class=\"vexgo-timeline-body\"><a class=\"vexgo-timeline-link\" href=\"/post/'+esc(p.slug)+'\">'+esc(p.title)+'</a><div class=\"vexgo-timeline-meta\"><span class=\"vexgo-timeline-date\">'+esc((p.createdAt||'').slice(0,10))+'</span>';if(p.excerpt){html+='<span class=\"vexgo-timeline-excerpt\">'+esc(p.excerpt)+'</span>'}html+='</div></div></li>'});html+='</ol></section>'});el.innerHTML=html}).catch(function(){el.innerHTML='<p class=\"vexgo-timeline-empty\">Failed to load timeline.</p>'})})();"
          }
        </script>
      </body>
    </html>
  );
}
