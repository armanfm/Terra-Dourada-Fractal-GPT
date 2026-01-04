## Roadmap (Terra Dourada Fractal GPT)

> Goal: evolve Terra Dourada Fractal GPT into a **sovereign, offline-first, deterministic** memory + retrieval + modular “experts-by-domain” engine — while preserving auditability and full user control.

---

### Phase 0 — Current (what already exists)
- Binary memory (`mind.bin`) as the single source of truth
- Deterministic “training” via canonical entries (no probabilistic ML)
- Recall + chat connected directly to the binary memory
- Semantic structures (e.g., `SemanticMemory`, entries, persistence)
- Execution logs/metrics (timing, similarity, etc.)

---

### Phase 1 — Memory Contract (stable binary format)
- Specify a fixed `mind.bin` **header**:
  - format version
  - build/training timestamp
  - entry count
  - checksum (integrity)
  - feature flags
- Compatibility guarantees:
  - version migrations
  - fallback readers for older files
- CLI tools:
  - `mind_validate`
  - `mind_dump --txt`
  - `mind_stats`

Deliverable: reliable, versioned, auditable memory.

---

### Phase 2 — Canonical Training (operators, not repetition)
- Formalize canonical operators:
  - affirmation
  - negation
  - restriction
  - condition
  - exclusivity
  - cause/effect
  - definition
- Standardize “train-by-topic”:
  - 1 topic → a set of canonical declarations
  - redundancy control (avoid spam repetition)
- Training profiles:
  - “quick” (fast)
  - “deep” (more operators)

Deliverable: topic specialization without probabilistic learning.

---

### Phase 3 — Deterministic Modular RAG (experts by domain)
- Split into multiple domain memories:
  - `mind_history.bin`
  - `mind_law.bin`
  - `mind_brands.bin`
  - `mind_terra_dourada.bin`
- Implement a deterministic router:
  - classify the topic
  - choose the correct mind(s)
  - run recall only inside that scope
- Enable composition:
  - 1 question → 2+ experts (when needed)

Deliverable: scale by domain, less noise, better precision.

---

### Phase 4 — Indexing & Performance (100k → 1M+ entries)
- Add lightweight indexes inside the binary:
  - prefix/token buckets
  - offsets for partial scanning
- Query caching:
  - normalized query → top-k results
- Parallel recall (multi-core)
- “Streaming recall” mode:
  - return top results incrementally

Deliverable: stable speed on very large memories.

---

### Phase 5 — Attention & Context (deterministic, explainable)
- Consolidate `atencao.rs` / `ContextVector`:
  - intent (question, command, comparison)
  - time (now vs historical)
  - style (short vs long-form)
  - domain (which mind)
- Deterministic ranking adjustments:
  - boosts by intent
  - penalties for noise
  - explicit, auditable rules

Deliverable: better output quality without turning into a black box.

---

### Phase 6 — True “Fractal” Memory (hierarchical structure)
- Organize memory into levels:
  - **core** (definitions)
  - **expansions** (details)
  - **examples** (cases)
  - **contrasts** (negations / anti-examples)
- Explicit links:
  - parent/child
  - related
  - contradiction
- Visualization tooling:
  - `mind_graph --topic X`

Deliverable: much stronger navigation and explainability.

---

### Phase 7 — Security, Integrity, Sovereignty
- Memory signing:
  - hash + signature (PQC if desired)
- Auditing:
  - append-only training logs
  - recall evidence trail
- “Private dignity / public verifiability” split:
  - separate public metadata from private content

Deliverable: sovereign memory that can be verified.

---

### Phase 8 — Productization (UX + integration)
- Chat UI with:
  - recall mode
  - training mode
  - evidence / provenance view
- Export:
  - TXT / JSON / PDF
- Integrations:
  - local API
  - plugin/bridge for apps

Deliverable: real usability for real users.

---
