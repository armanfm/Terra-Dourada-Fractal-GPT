## Roadmap (Terra Dourada Brands)

> Goal: evolve from a deterministic segment-scoped pre-analysis engine into a practical, scalable workflow for real trademark datasets — without turning it into a “black box”.

### Phase 0 — Current (What already works)
- Segment-scoped `mind.bin` loading (one segment at a time)
- Deterministic similarity engine (Bytes + SHA256) with:
  - `winner_pct = max(bytes_pct, sha256_pct)`
  - best-match retrieval + minimal output
- Web UI + API endpoints for load + check
- Deterministic, auditable results (no ML training)

---

### Phase 1 — Data Pipeline (Make `mind.bin` repeatable)
- Define a **build spec** for each segment:
  - source file(s)
  - cleaning rules
  - normalization rules (including accent strategy)
  - dedup strategy (exact duplicates only)
- Add a **builder CLI**:
  - `build_segment --input data.csv --segment cosmetics --out cosmetics.bin`
- Add **version metadata inside the binary**:
  - segment name
  - build timestamp
  - source identifier
  - entry count
  - checksum/hash of the file
- Add optional **TXT export**:
  - `export_txt cosmetics.bin > cosmetics.txt`

Deliverable: fully reproducible, versioned dataset builds.

---

### Phase 2 — Normalization Policy (Accents, casing, punctuation)
- Standardize a clear policy:
  - store both **raw_name** and **normalized_name**
  - normalization applied only for comparison, not for display
- Implement deterministic normalization rules:
  - case folding
  - whitespace collapsing
  - punctuation handling
  - accent folding (optional but recommended)
- Report includes both:
  - display name (original)
  - analysis name (normalized)

Deliverable: consistent results with diacritics and minor variants.

---

### Phase 3 — Performance & Scale (Large segments)
- Add a lightweight **pre-filter** to avoid scanning everything:
  - prefix buckets (first 1–3 characters)
  - token buckets (first token / root token)
- Add a compact **index table** in the binary:
  - bucket → [start_offset, count]
- Optional parallel scanning (multi-core) for big segments

Deliverable: stable speed for 100k+ / 1M+ names per segment.

---

### Phase 4 — Output & Explainability (Without “magic scores”)
- Add “why this match” fields (still deterministic):
  - top shared substrings length
  - token overlap summary
  - normalization steps applied
- Add multi-result output (Top-K):
  - return top 5 conflicts, not only top 1
- Add report export modes:
  - JSON (API)
  - TXT (human audit)
  - CSV (batch review)

Deliverable: reviewers can understand matches quickly.

---

### Phase 5 — Batch Mode (Real operational workflow)
- Upload a list (CSV/TXT) and get batch results:
  - `batch_check cosmetics.bin brands.csv -> results.csv`
- Rate limiting and caching for repeated queries
- UI: “upload list” + downloadable report

Deliverable: practical tool for agencies and teams.

---

### Phase 6 — Semantic Layer (Optional, explicit)
- Add a clearly separated step:
  - “Technical result” → “Semantic review”
- If using LLM:
  - it only consumes the metrics + candidate names
  - outputs a **Recommendation**, never “legal decision”
- Provide templates:
  - “HIGH RISK reasons”
  - “GRAY ZONE questions”
  - “LOW SIGNAL notes”

Deliverable: semantic reasoning becomes standardized and safe.

---

### Phase 7 — Dataset sourcing (Later, if desired)
- Add documented importers for public sources
- Add update schedule as “snapshots” (weekly/monthly)
- Maintain changelog per segment:
  - added/removed names
  - counts by source

Deliverable: continuous improvement while preserving determinism.

---

## Suggested next 3 concrete tasks (fast wins)
1. **Add raw vs normalized** names (accent folding + casing) and keep original display
2. **Top-K matches** (return top 5) + TXT export
3. **Binary metadata header** (segment, timestamp, count, checksum)

These three make the project immediately stronger for real-world use and hackathon judging.
