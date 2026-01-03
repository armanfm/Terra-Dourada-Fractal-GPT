# Terra Dourada Brands

**Live demo:** https://terra-dourada-gpt-green-butterfly-3484.fly.dev/

## Overview

**Terra Dourada Brands** is a deterministic, segment-scoped engine for **technical pre-analysis of trademark name similarity**.

It fills the gap between:
- **official trademark search** (data listing + literal substring matching)
and
- **human interpretation** (risk of confusion, phonetics, semantics, market context).

The system **does not replace** official authorities and **does not** register trademarks.  
It provides **fast, reproducible technical evidence** so humans can decide more confidently.

---

## The Problem

Official trademark search interfaces rely primarily on **literal substring matching**.

This causes:
- noisy, unranked result lists
- unrelated terms appearing together
- confusion between active/expired and irrelevant classes
- slow manual filtering and interpretation

In practice, trademark evaluation becomes slow, noisy, and error-prone.  
There is no practical “technical pre-analysis layer” before official checks.

---

## What Terra Dourada Brands Solves

Terra Dourada Brands introduces a **deterministic pre-analysis layer**:

- runs inside a **chosen segment scope** (e.g., cosmetics)
- compares a proposed name against **the segment universe**
- returns **the closest conflict candidate**
- exposes **two deterministic metrics**
- enables an optional **semantic review** step (human/LLM)

It turns raw search into actionable relevance.

---

## Core Principles

- **Deterministic:** same input + same memory = same result  
- **Segment-scoped:** similarity only makes sense inside a market segment  
- **Metric-based:** no embeddings, no opaque scores, no ML training  
- **Human-in-the-loop:** the engine informs; humans interpret and decide  

---

## “Training” Means Building `mind.bin` (Not Machine Learning)

In Terra Dourada Brands, **training is not ML training**.

There is:
- no model fitting
- no probabilistic inference
- no adaptive behavior

**Training means building a deterministic memory file (`mind.bin`)** from existing trademark datasets:

1. Collect existing marks (public exports / curated lists)  
2. Filter by **segment**  
3. Clean & deduplicate entries (within the segment)  
4. Store in a compact binary memory file (`mind.bin`)  

Each `mind.bin` is a **versioned snapshot** of a segment universe.

---

## Segment-Scoped Memory (Mandatory Design)

Each segment has its own memory file, for example:

- `cosmetics.bin`
- `pharmaceutical.bin`
- `technology.bin`

Only one segment memory is loaded during analysis.

Why:
- reduces noise and false conflicts across unrelated markets
- keeps comparisons meaningful
- scales better than “one file for everything”

---

## How Similarity Works (Two Deterministic Metrics)

For each candidate in the segment, the engine computes:

### 1) Byte Similarity (`bytes_pct`)

Captures **orthographic / visual structure**:
- shared substrings
- truncations and abbreviations
- spacing/punctuation patterns
- length/shape similarity

### 2) SHA256-Derived Similarity (`sha256_pct`, lossy)

Provides a **second deterministic signal**:
- highlights structural divergence under transformations
- reduces over-reliance on pure surface appearance

> Note: This is not “semantic AI”. It is a deterministic transformation-based metric.

---

## Selection Rule (No Averages)

For each candidate:

- compute `bytes_pct`
- compute `sha256_pct`
- define: **`winner_pct = max(bytes_pct, sha256_pct)`**

The system selects the candidate with the **highest `winner_pct`** and returns:

- candidate name
- `bytes_pct`
- `sha256_pct`
- `winner_pct`
- `winner_metric` (`bytes` or `sha256`)

**No averaging. No combined score. No hidden weights.**

---

## Output Formats

### JSON (API)

Minimal structure:

- `query`
- `segment` (implicit via loaded memory)
- `best_match`
- `bytes_pct`
- `sha256_pct`
- `winner_pct`
- `winner_metric`

### TXT (Export / Human Review)

Results can be printed/exported as plain text for analysis and record keeping.

---

## Two-Layer Architecture (Correct Separation)

### Layer A — Technical Pre-Analysis (Engine)

Deterministic, auditable, reproducible.

**Answers:**  
> “What is the closest existing mark in this segment?”

### Layer B — Semantic Review (Human or Optional LLM)

Interpretive, contextual, non-legal.

**Answers:**  
> “Given the closest match and metrics, is this likely confusing in the real world?”

This separation is intentional:
- calculation stays deterministic
- semantics remain explicit (no fake authority)

---

## Triage Policy (Non-Legal, Project-Level)

This is NOT law. It is a practical triage guideline:

- **winner_pct ≥ 90%** → **Direct collision signal** (high priority flag)
- **70%–89.99%** → **Gray zone** (requires semantic review)
- **< 70%** → **Lower signal** (still possible conflict, but lower priority)

---

# Examples (Technical → Semantic)

## Example 1 — Direct Collision (Identical Name)

### Technical Pre-Analysis

Query: "terra dourada"
Best match: "Terra Dourada"
bytes_pct: 98.43%
sha256_pct: (depends)
winner_pct: 98.43% (bytes)
winner_metric: bytes

yaml
Copiar código

### Semantic Review (Non-Legal Recommendation)

**Recommendation:** HIGH RISK (direct collision)  
**Reason:** same core name, indistinguishable visually/phonetic, strong association risk.

**Disclaimer:** Technical pre-analysis only. Final decision is human/legal.

---

## Example 2 — High Similarity (A Boticaria vs O Botic)

### Technical Pre-Analysis

Query: "A Boticaria"
Best match: "O Botic"
bytes_pct: 86.72%
sha256_pct: 60.16%
winner_pct: 86.72% (bytes)
winner_metric: bytes

yaml
Copiar código

### Semantic Review (Non-Legal Recommendation)

**Recommendation:** HIGH RISK  
**Reason:** strong visual overlap (“Botic” root), minor grammatical variation (“A” vs “O”), likely consumer association with the “Botic…” ecosystem.

**Disclaimer:** Not a legal decision. Assistive risk assessment only.

---

## Example 3 — Gray Zone With Semantic Divergence (terra amassada vs Terra Dourada)

### Technical Pre-Analysis

Query: "terra amassada"
Best match: "Terra Dourada"
bytes_pct: 86.71%
sha256_pct: (depends)
winner_pct: 86.71% (bytes)
winner_metric: bytes

yaml
Copiar código

### Semantic Review (Non-Legal Recommendation)

**Recommendation:** POSSIBLE APPROVAL (needs context)  
**Reason:** shared generic term “Terra”, but strong conceptual divergence between “Dourada” (value/brightness) and “Amassada” (texture/physical state). Confusion risk may be lower depending on segment and presentation.

**Disclaimer:** Final outcome depends on human/legal review.

---

# Considerations & FAQ

## 1) Memory Updates (`mind.bin`)

Currently, each `mind.bin` is produced as a **versioned snapshot**.

- updates happen by rebuilding the segment file from public exports/curated lists
- each build produces a new file version
- preserves reproducibility and auditability

Automation can be added later, but the current design prioritizes deterministic snapshots.

---

## 2) Deterministic Metrics (Trust Without “Magic Scores”)

Trust comes from:
- deterministic metrics
- reproducible outputs
- explicit separation between technical calculation and semantic interpretation
- minimal, auditable reporting

---

## 3) Hybrid Brands (Multi-Segment)

A brand can appear in multiple segment memories if relevant.

- deduplication occurs **within each segment**
- cross-segment duplication is expected and correct

---

## 4) Scalability

For typical segment sizes (thousands to tens of thousands), sequential scanning is fast.

For very large segments, future optimizations are possible:
- prefix/token bucketing
- small index table in binary
- parallel scanning
- caching/memory mapping

Current focus: correctness + reproducibility.

---

## 5) Why TXT Output Matters

Binary memory ensures fast deterministic scanning, while TXT output supports:
- manual review
- team sharing
- attaching analysis notes
- later semantic review (human/LLM)

---

## What Terra Dourada Brands Is NOT

- ❌ Not a trademark registry  
- ❌ Not a legal decision engine  
- ❌ Not an INPI replacement  
- ❌ Not probabilistic AI judgment  

It is a **technical pre-analysis and decision-support layer**.

---

## Key Insight

Official systems primarily **list data** (often via substring matching).  
Terra Dourada Brands **organizes relevance** within a scoped segment, using deterministic metrics and explicit huma
