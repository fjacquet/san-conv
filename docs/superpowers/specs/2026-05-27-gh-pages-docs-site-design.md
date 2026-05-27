# Design: Documentation site on GitHub Pages (MkDocs Material)

- **Date:** 2026-05-27
- **Status:** Approved (brainstorming)
- **Topic:** Publish the project's existing Markdown documentation as a searchable static site on GitHub Pages.

## Goal

Turn the Markdown already in the repo into a polished, searchable documentation site at
`https://fjacquet.github.io/san-conv/`, built and deployed automatically from `main`.
No change to the shipped binary; documentation tooling is a CI-only concern (Python),
consistent with the project's "single Go binary, no runtime deps" constraint.

## Decisions (from brainstorming)

1. **Generator:** MkDocs + Material for MkDocs.
2. **Scope:** Everything user-facing in `docs/` plus root `README`/`CHANGELOG`/`SECURITY`.
   The internal GSD planning artifacts under `docs/superpowers/` are **excluded** from the
   published site (kept in the repo).
3. **Deploy:** GitHub Actions → GitHub Pages using the artifact flow
   (`actions/upload-pages-artifact` + `actions/deploy-pages`), Pages source = "GitHub Actions".
   No `gh-pages` branch.

## Site structure (navigation)

| Nav entry | Source file | Notes |
|-----------|-------------|-------|
| Home | `docs/index.md` (new) | Bespoke landing page: what san-conv is, install one-liner, links into sections. Not a copy of README (README keeps repo-relative links that don't resolve on the site). |
| User Guide | `docs/USER_GUIDE.md` | As-is. |
| Reference → PRD | `docs/PRD.md` | As-is. |
| Reference → Security | `docs/security.md` (new stub) | Includes root `SECURITY.md` via snippet. |
| Design Decisions | `docs/adr/0001…0011-*.md` | 11 ADRs, listed explicitly in nav. |
| Changelog | `docs/changelog.md` (new stub) | Includes root `CHANGELOG.md` via snippet. |

**Single-source-of-truth for root files:** `README.md`, `CHANGELOG.md`, and `SECURITY.md`
stay at the repo root (GitHub renders them there). The site pulls `CHANGELOG.md` and
`SECURITY.md` into thin stub pages using `pymdownx.snippets` (`--8<-- "CHANGELOG.md"`),
with `base_path: ["."]` so the extension can read files above `docs/`. The home page is a
new, bespoke `docs/index.md` to avoid README's repo-relative link breakage.

## Files to create / change

```
mkdocs.yml                       # site config (repo root)
requirements-docs.txt            # pinned: mkdocs-material (pulls mkdocs + pymdown-extensions)
docs/index.md                    # landing page (new)
docs/changelog.md                # stub: --8<-- "CHANGELOG.md"
docs/security.md                 # stub: --8<-- "SECURITY.md"
.github/workflows/docs.yml       # build (+ strict) and deploy
.gitignore                       # add: site/   (mkdocs build output)
```

No existing files move. `docs/adr/*`, `docs/USER_GUIDE.md`, `docs/PRD.md` are used as-is.

## mkdocs.yml (shape)

```yaml
site_name: san-conv
site_description: Convert SAN zoning configs between Cisco MDS (NX-OS) and Brocade FOS
site_url: https://fjacquet.github.io/san-conv/
repo_url: https://github.com/fjacquet/san-conv
repo_name: fjacquet/san-conv
edit_uri: edit/main/docs/
docs_dir: docs

theme:
  name: material
  palette:           # light/dark toggle
    - scheme: default   # (+ toggle to slate)
  features:
    - navigation.sections
    - navigation.top
    - navigation.instant
    - search.suggest
    - content.code.copy
    - content.action.edit
    - content.action.view
    - toc.follow

plugins:
  - search

markdown_extensions:
  - admonition
  - tables
  - toc: { permalink: true }
  - pymdownx.highlight
  - pymdownx.inlinehilite
  - pymdownx.snippets: { base_path: ["."], check_paths: true }
  - pymdownx.superfences:
      custom_fences:
        - name: mermaid
          class: mermaid
          format: !!python/name:pymdownx.superfences.fence_code_format

exclude_docs: |
  superpowers/

nav:
  - Home: index.md
  - User Guide: USER_GUIDE.md
  - Reference:
      - PRD: PRD.md
      - Security: security.md
  - Design Decisions:
      - adr/0001-go-single-binary.md
      # … 0002 … 0011
  - Changelog: changelog.md
```

Versions (mkdocs-material, action tags) are pinned to current releases at implementation
time, validated against upstream — not guessed.

## Build & deploy flow (`.github/workflows/docs.yml`)

- **Triggers:** push to `main` touching docs-relevant paths
  (`docs/**`, `mkdocs.yml`, `README.md`, `CHANGELOG.md`, `SECURITY.md`,
  `requirements-docs.txt`, the workflow itself); `pull_request` on the same paths;
  `workflow_dispatch`.
- **Permissions:** `contents: read`, `pages: write`, `id-token: write`.
- **Concurrency:** group `pages`, `cancel-in-progress: false`.
- **build job (all triggers):** checkout → `actions/setup-python` (3.x) → cache `~/.cache`
  → `pip install -r requirements-docs.txt` → `mkdocs build --strict --site-dir site`
  → `actions/upload-pages-artifact` (path `site`). `--strict` fails the build on broken
  links or pages missing from nav, so PRs catch breakage before merge.
- **deploy job:** `needs: build`, runs only when `github.event_name != 'pull_request'`
  and `github.ref == 'refs/heads/main'`; `environment: github-pages`;
  `actions/deploy-pages`.
- **One-time setup:** enable Pages with the GitHub-Actions source
  (`gh api -X POST repos/fjacquet/san-conv/pages -f build_type=workflow`, or repo Settings → Pages).

## Error handling

- Builds run with `--strict`: any dead internal link, missing nav target, or broken
  snippet include fails CI rather than publishing a broken site.
- PR builds never deploy, so a bad docs change cannot reach the live site.

## Testing / verification

- Local: `pip install -r requirements-docs.txt && mkdocs build --strict` (and
  `mkdocs serve` for live preview) before pushing.
- CI: strict build on every PR; deploy only from `main`.
- Post-deploy: confirm the site loads at the Pages URL and that `docs/superpowers/`
  pages are absent (exclusion working).

## Out of scope (YAGNI)

- Versioned docs (`mike` / version selector) — single "latest" site.
- Custom domain — use the default `*.github.io` URL.
- Blog, tags, i18n, social cards, search analytics, git-committers/authors plugins.
- Auto-generated API/CLI reference — the hand-written USER_GUIDE covers usage.

## Open questions

None — all resolved during brainstorming.
