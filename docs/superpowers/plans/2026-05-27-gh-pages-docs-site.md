# GitHub Pages Documentation Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish the repo's existing Markdown as a searchable MkDocs Material site at `https://fjacquet.github.io/san-conv/`, built and deployed from `main` via GitHub Actions.

**Architecture:** A root `mkdocs.yml` drives Material for MkDocs over `docs_dir: docs`. Existing files (USER_GUIDE, PRD, 11 ADRs) are used as-is; root `CHANGELOG.md`/`SECURITY.md` are pulled into thin stub pages via `pymdownx.snippets`; a bespoke `docs/index.md` is the landing page. The internal `docs/superpowers/` tree is removed from the build with native `exclude_docs`. A `docs.yml` workflow builds with `--strict` on every PR and deploys to Pages (artifact flow — no `gh-pages` branch) only from `main`.

**Tech Stack:** MkDocs + `mkdocs-material==9.7.6` (Python, CI-only), GitHub Actions (`actions/upload-pages-artifact@v5`, `actions/deploy-pages@v5`), GitHub Pages (source = "GitHub Actions", already enabled).

**Prerequisite (already done by the user):** GitHub Pages is enabled with the "GitHub Actions" source. No `gh api .../pages` call needed.

---

## Task 1: Build the documentation site locally

Produce a complete, strictly-valid site that builds on a developer machine before any CI is involved. The acceptance test for every step in this plan is `mkdocs build --strict`, which fails on broken links, missing-from-nav pages, or unresolved snippet includes.

**Files:**
- Create: `requirements-docs.txt`
- Create: `mkdocs.yml`
- Create: `docs/index.md`
- Create: `docs/changelog.md`
- Create: `docs/security.md`
- Modify: `.gitignore` (append `site/`)
- Used as-is (no change): `docs/USER_GUIDE.md`, `docs/PRD.md`, `docs/adr/0001…0011-*.md`, root `CHANGELOG.md`, root `SECURITY.md`

- [ ] **Step 1: Pin the docs toolchain**

Create `requirements-docs.txt`:

```text
mkdocs-material==9.7.6
```

(`mkdocs-material` pulls in `mkdocs` and `pymdown-extensions`, which provide `superfences`, `snippets`, and `highlight` — no other packages needed.)

- [ ] **Step 2: Write the site configuration**

Create `mkdocs.yml` at the repo root:

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
  icon:
    repo: fontawesome/brands/github
  palette:
    - media: "(prefers-color-scheme: light)"
      scheme: default
      toggle:
        icon: material/brightness-7
        name: Switch to dark mode
    - media: "(prefers-color-scheme: dark)"
      scheme: slate
      toggle:
        icon: material/brightness-4
        name: Switch to light mode
  features:
    - navigation.sections
    - navigation.top
    - navigation.instant
    - search.suggest
    - search.highlight
    - content.code.copy
    - content.action.edit
    - content.action.view
    - toc.follow

plugins:
  - search

markdown_extensions:
  - admonition
  - tables
  - toc:
      permalink: true
  - pymdownx.highlight
  - pymdownx.inlinehilite
  - pymdownx.snippets:
      base_path: ["."]
      check_paths: true
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
      - adr/0002-warn-and-continue.md
      - adr/0003-bidirectional-mds-primary.md
      - adr/0004-ir-compiler-pipeline.md
      - adr/0005-two-pass-mds-parser.md
      - adr/0006-fos-naming-default.md
      - adr/0007-cobra-cli-framework.md
      - adr/0008-mds-smart-zoning-to-peer-zoning.md
      - adr/0009-peer-zone-consolidation.md
      - adr/0010-peerzone-roundtrip.md
      - adr/0011-brocade-flat-to-mds-smart.md
  - Changelog: changelog.md
```

(ADR nav entries list paths only; MkDocs derives each title from the file's `#` H1.)

- [ ] **Step 3: Write the landing page**

Create `docs/index.md`:

````markdown
# san-conv

**Convert SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS.**

`san-conv` is a single, dependency-free Go binary for ops teams migrating zoning between
Cisco MDS and Brocade switches. Give it a full MDS running-config and it produces
ready-to-apply Brocade FOS CLI commands — with clear warnings for anything it cannot
convert cleanly. Conversion is bidirectional.

## Install

Download a prebuilt binary from the
[latest release](https://github.com/fjacquet/san-conv/releases/latest), or build from
source (requires Go 1.21+):

```bash
go install github.com/fjacquet/san-conv@latest
```

## Quick start

```bash
# Cisco MDS running-config  ->  Brocade FOS commands
san-conv mds2brocade myconfig.txt

# Brocade FOS config  ->  Cisco MDS commands
san-conv brocade2mds myconfig.txt
```

See the **[User Guide](USER_GUIDE.md)** for flags, worked examples, and conversion details.

## Documentation

- **[User Guide](USER_GUIDE.md)** — installation, commands, flags, worked examples
- **[Product Requirements](PRD.md)** — what the tool does and why
- **[Design Decisions](adr/0001-go-single-binary.md)** — architecture rationale (ADRs)
- **[Changelog](changelog.md)** — release history
````

- [ ] **Step 4: Write the Changelog stub page**

Create `docs/changelog.md` (pulls in the canonical root file — single source of truth):

```text
--8<-- "CHANGELOG.md"
```

- [ ] **Step 5: Write the Security stub page**

Create `docs/security.md`:

```text
--8<-- "SECURITY.md"
```

- [ ] **Step 6: Ignore the build output**

Append to `.gitignore` (under the existing "Build artifacts" group):

```text
# MkDocs build output
site/
```

- [ ] **Step 7: Install the toolchain and build strictly**

Run:

```bash
python3 -m venv .venv-docs && . .venv-docs/bin/activate
pip install -r requirements-docs.txt
mkdocs build --strict --site-dir site
```

Expected: build completes with `INFO - Documentation built in …`, **zero** `WARNING`/`ERROR`
lines. A failure here means a broken link, a page missing from nav, or an unresolved
`--8<--` include — fix the offending file and re-run.

- [ ] **Step 8: Verify the snippet includes and exclusion resolved**

Run:

```bash
grep -rl "Keep a Changelog" site/changelog/ && \
test ! -d site/superpowers && echo "OK: changelog included, superpowers excluded"
```

Expected: prints a path under `site/changelog/` then `OK: changelog included, superpowers excluded`.
(Confirms `CHANGELOG.md` was inlined and `docs/superpowers/` did not reach the build.)

- [ ] **Step 9: Spot-check in a browser (optional but recommended)**

Run `mkdocs serve` and open `http://127.0.0.1:8000/`. Confirm: home page renders, search
works, the 11 ADRs appear under "Design Decisions", Changelog/Security pages show content,
and there is no "superpowers" section. Stop with Ctrl-C.

- [ ] **Step 10: Commit**

```bash
git add requirements-docs.txt mkdocs.yml docs/index.md docs/changelog.md docs/security.md .gitignore
git commit -m "docs(site): add MkDocs Material configuration and pages

Builds the docs/ tree into a searchable static site (home, User Guide, PRD,
Security, 11 ADRs, Changelog). Root CHANGELOG.md/SECURITY.md are pulled in via
pymdownx.snippets; docs/superpowers/ is excluded via exclude_docs.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: Build and deploy via GitHub Actions

Add CI that builds the site strictly on every PR and deploys to GitHub Pages only from `main`.

**Files:**
- Create: `.github/workflows/docs.yml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/docs.yml`:

```yaml
name: Docs

on:
  push:
    branches: [main]
    paths:
      - "docs/**"
      - "mkdocs.yml"
      - "README.md"
      - "CHANGELOG.md"
      - "SECURITY.md"
      - "requirements-docs.txt"
      - ".github/workflows/docs.yml"
  pull_request:
    paths:
      - "docs/**"
      - "mkdocs.yml"
      - "README.md"
      - "CHANGELOG.md"
      - "SECURITY.md"
      - "requirements-docs.txt"
      - ".github/workflows/docs.yml"
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-python@v6
        with:
          python-version: "3.x"
          cache: pip
          cache-dependency-path: requirements-docs.txt
      - run: pip install -r requirements-docs.txt
      - run: mkdocs build --strict --site-dir site
      - uses: actions/upload-pages-artifact@v5
        with:
          path: site

  deploy:
    name: Deploy
    needs: build
    if: github.event_name != 'pull_request' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - id: deployment
        uses: actions/deploy-pages@v5
```

- [ ] **Step 2: Security-scan the workflow**

Scan `.github/workflows/docs.yml` with the semgrep MCP tool (`semgrep_scan`). Expected:
0 findings. (Project policy: scan generated workflow/config before committing.)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/docs.yml
git commit -m "ci(docs): build site strictly on PRs, deploy to Pages from main

Artifact-flow deploy (upload-pages-artifact + deploy-pages), no gh-pages branch.
PR builds run mkdocs build --strict but never deploy.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

- [ ] **Step 4: Push and watch the deploy**

```bash
git push origin main
RUN=$(gh run list --workflow=Docs --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$RUN" --exit-status --interval 15
```

Expected: both `Build` and `Deploy` jobs succeed.

- [ ] **Step 5: Verify the live site**

```bash
curl -sSfo /dev/null -w "%{http_code}\n" https://fjacquet.github.io/san-conv/
curl -sS https://fjacquet.github.io/san-conv/superpowers/ -o /dev/null -w "superpowers -> %{http_code}\n"
```

Expected: home returns `200`; the `superpowers/` path returns `404` (confirming exclusion).
Open the site and confirm search and the ADR section render.

---

## Self-Review

**1. Spec coverage:**
- Generator = MkDocs Material → Task 1 Step 2, `requirements-docs.txt`. ✓
- Content = index + USER_GUIDE + PRD + Security + 11 ADRs + Changelog → Task 1 nav + stub pages. ✓
- Exclude `docs/superpowers/` → `exclude_docs` (Task 1 Step 2), verified Task 1 Step 8 / Task 2 Step 5. ✓
- Root files via snippet includes, single source of truth → Task 1 Steps 4–5. ✓
- Bespoke landing page (not README copy) → Task 1 Step 3. ✓
- Deploy via Actions artifact flow, no `gh-pages` → Task 2 Step 1. ✓
- Strict build on PRs, deploy only from main → Task 2 Step 1 (`--strict`, deploy `if:`). ✓
- Pages source already enabled → noted in prerequisite. ✓
- Out of scope (mike/custom domain/blog) → none added. ✓

**2. Placeholder scan:** No TBD/TODO; every file has complete content; commands have expected output. ✓

**3. Type/name consistency:** `site_dir` is `site` everywhere (config flag, artifact `path`, gitignore, verification globs); workflow name `Docs` matches `gh run list --workflow=Docs`; nav paths match created/existing files. ✓
