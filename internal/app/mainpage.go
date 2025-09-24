package app

// mainPageHTML returns a curated article for the EndlessWiki main page.
func mainPageHTML() string {
    return `
<h1>EndlessWiki</h1>
<div class="endlesswiki-body">
  <p><strong>EndlessWiki</strong> is a continuously generated encyclopedia that pairs on-demand writing with durable storage. Inspired by <a href="/wiki/wikipedia">Wikipedia</a>, it keeps familiar navigation while relying on modern infrastructure to fill every gap in the knowledge graph.</p>

  <h2>System overview</h2>
  <p>The service is written in <a href="/wiki/go_programming_language">Go</a>, served as a minimal HTTP application, and stores finished articles in <a href="/wiki/mysql">MySQL</a>. When a request arrives for an unseen slug, EndlessWiki calls Groq's <code>moonshotai/kimi-k2-instruct-0905</code> model to draft an HTML article, which is then persisted for all future visitors.</p>

  <h2>Request pipeline</h2>
  <ul>
    <li>Normalize the slug (spaces, Unicode, punctuation) and check for an existing record.</li>
    <li>Generate the article through Groq if the slug is new, otherwise serve the cached version immediately.</li>
    <li>Render the content with a Wikipedia-style template that surfaces navigation, search, and archive metadata.</li>
  </ul>

  <h2>Explore more</h2>
  <div class="mainpage-columns">
    <section>
      <h3>Architecture references</h3>
      <ul>
        <li><a href="/wiki/groq">Groq</a> inference APIs used for content generation.</li>
        <li><a href="/wiki/mysql">MySQL</a> storage engine backing the page archive.</li>
        <li><a href="/wiki/railway_platform">Railway platform</a> for deployment and environment management.</li>
      </ul>
    </section>
    <section>
      <h3>Operational tools</h3>
      <ul>
        <li>Scan the index with the <a href="/search?q=endless">search interface</a>.</li>
        <li>Jump to a <a href="/random">random page</a>.</li>
        <li>Resume with the <a href="/recent">most recently generated article</a>.</li>
      </ul>
    </section>
    <section>
      <h3>Further reading</h3>
      <ul>
        <li>Concepts in <a href="/wiki/information_retrieval">information retrieval</a>.</li>
        <li>Trends in <a href="/wiki/artificial_intelligence">artificial intelligence</a>.</li>
        <li>How open documentation evolved on <a href="/wiki/open_source">open source</a> platforms.</li>
      </ul>
    </section>
  </div>
</div>`
}
