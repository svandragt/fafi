# DESIGN.md

Design decisions for the fafi search results UI (`pub/index.html`). The page
is a single screen — a list of bookmarks the user is scanning, filtering, and
acting on. Every decision below pulls in the same direction: **chrome
disappears into the list**.

## Register

Product, not brand. The UI serves a tool the user already chose to install;
it is not selling anything. Optimise for repeat-use scanning, not first
impression.

## Through-line

The page is a list of text. Anything that isn't text or a direct affordance
on the text should fade. Framework out, cards out, default-state pills out,
sidebar out, stretched rows out.

## Color

OKLCH tokens, light theme, neutrals tinted toward hue 250 (cool blue-grey)
with chroma 0.005–0.012. Never `#fff` or `#000`.

- `--bg` `oklch(0.985 0.004 85)` — page
- `--surface` `oklch(1 0 0)` — input/panel backgrounds
- `--border` / `--border-strong` — hue 250, chroma 0.005–0.008
- `--text` / `--text-muted` / `--text-subtle` — hue 250, chroma 0.01–0.012
- `--link` `oklch(0.45 0.17 255)` — the one cool accent
- `--mark` `oklch(0.94 0.1 95)` — the **only** warm color on the page,
  reserved for FTS match highlighting

**Strategy:** Restrained. Tinted neutrals plus one cool accent (links) and
one warm accent (mark). The warm/cool split is deliberate: highlighted
matches are the one thing that must pop.

## Theme

Light. The scene is someone triaging their own bookmarks during the day,
reading long-form snippet text. Dark mode would punish the snippet-heavy
content this UI exists to surface.

## Typography

System font stack. Body at `0.9375rem / 1.55`. Hierarchy through weight +
size, not color. Title link is the only element with real weight; everything
else recedes.

## Layout

- Column width `min(48rem, calc(100% - 2.5rem))`. Keeps title + snippet
  inside the 65–75ch comfort zone for vertical scanning.
- Rows pack from the top (`align-content: start` on the body grid). Sparse
  result pages must not stretch rows to fill the viewport — that reads as
  "broken/empty," not "intentional."
- No cards, no per-row shadows. Rows are separated by a single bottom
  border. Density comes from removing chrome, not from shrinking padding.

## Information order in a result row

1. Small metadata strip: URL, pills (when present), hover actions
2. Title link (the visual anchor)
3. Snippet (plain text, click-to-expand)

The URL goes **above** the title, not below. Scanning bookmarks is often
"where did this come from?" — the domain answers that faster than the title.
Title still wins visually via weight and size.

## Pills

Tinted backgrounds with matching darker text — "tag" register, not "alert"
register. Solid-color status blocks are banned here; they shout.

**The default state does not render a pill.** A label that appears on 99% of
rows is noise. Pills appear only for deviations: 4xx, 5xx, pending, etc. The
absence of a pill *is* the indexed/2xx state.

## Filters

Horizontal chip row above the results, not a sidebar. A sidebar on a
single-purpose page steals horizontal budget from the content. The chip row
sits in space that was already empty above the list, and each chip carries
its own count so the filter doubles as a glance-able status summary.

## Search match highlighting

`<mark>` is the one warm color on the page (hue 95). Used for FTS matches in
titles, URLs, and snippets. Nothing else may use warm hues — that exclusivity
is what makes the highlight readable at a glance.

## Snippets

Click-to-expand. Density wins as the default; depth is one click away. **No
modal.** Expansion happens inline. Modals are reserved for destructive
confirmations (soft delete) that need an explicit yes.

## Motion

Negligible. The page is not animated. Hover states are color/opacity only.
Row removal on soft delete is instant. We do not animate layout properties.

## Bans specific to this project

- **Side-stripe borders** on result rows. Full bottom border or nothing.
- **Solid-color status blocks.** All pills are tinted tags.
- **Cards around result rows.** A list is not a deck.
- **Sidebars** on a single-purpose page.
- **A pill for the default state.** If everything has it, no one sees it.
- **Stretching rows to fill the viewport.** Sparse pages stay sparse.
- **Warm accent colors** anywhere except `<mark>`. Highlight exclusivity
  is non-negotiable.
- **Modals** except for irreversible confirmations.

## What the UI does NOT do (and why)

- **No dark mode toggle.** Long-form snippet reading is the load-bearing
  task. Adding a toggle invites half-tuned dark styles; pick one and tune
  it.
- **No hero metric.** This is not a dashboard.
- **No gradient anything.** Single solid colors only.
- **No icons in result rows beyond content-type.** The content-type icon
  is the only iconography — it carries information the text can't.
