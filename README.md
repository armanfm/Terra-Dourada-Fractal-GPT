
<img width="1280" height="720" alt="image" src="https://github.com/user-attachments/assets/246d60cc-d4d6-48a1-b31b-1c719d75a936" />

# Terra-Dourada-Fractal-GPT

Terra Dourada is a deterministic semantic memory engine that performs recall **without embeddings** or external recall models.  
It combines structural similarity, token semantics, redundancy control, and sovereign hashing to produce **auditable, reproducible** results.

This project focuses on **identity preservation**, **semantic stability**, and **transparent reasoning**, avoiding probabilistic black-box recall.

---

## What is Terra Dourada?

Terra Dourada is **not** a generative model.

It is a **semantic recall engine** designed to:
- store meaning deterministically
- retrieve relevant context without guessing
- preserve identity and structure across long interactions

Generative models (LLMs) can be layered on top, but **recall is sovereign**.

---

## How it works

1. Text is ingested into a semantic memory store.
2. Each entry is indexed deterministically (no embeddings).
3. Recall uses a multi-stage semantic funnel:
   - token focus extraction
   - informational token weighting
   - fast prefilters (cheap → expensive)
   - structural similarity (bytes / hash / base64)
   - redundancy filtering (Jaccard)
4. Results are ranked deterministically and reproducibly.

Given the same input and memory, the output is **always the same**.

---

## Getting started

### Requirements
- Rust (stable)
- Cargo

### Build
~~~bash
cargo build --release
~~~

### Run (example)
~~~bash
cargo run --release
~~~

> The current implementation exposes the semantic recall engine as a Rust library.  
> You can integrate it into your own applications or build a CLI / server on top of it.

---

## LLM integration (optional)

Terra Dourada does **not** require an API key to perform semantic recall.

However, when layered with a generative model (e.g. Gemini or other LLMs), an API key is required **only for response fluency and text rendering**.

In this setup:
- recall remains deterministic and local
- the LLM does not decide meaning or ranking
- the LLM is used solely to improve readability and coherence

Without an API key, the system still functions correctly, returning raw recall results.

---

## Training pipeline (implemented, instant semantic training)

The training + ingestion pipeline is **already implemented**.

A key differentiator is that **semantic training is instantaneous**: new text can be incorporated into memory immediately, without waiting for embedding jobs or long training cycles.

What remains is deployment as a separate service (deployment target: Fly.io), so training can run remotely while recall stays local and deterministic.

This separation is intentional:
- training can scale independently
- recall remains fast, offline-capable, and reproducible
- inference does not depend on remote services

---

## Design principles

- **No embeddings**
- **No probabilistic recall**
- **No opaque ranking**
- **Deterministic by default**
- **Auditable and reproducible**
- **Identity-aware via stable recall**
- **Instant semantic training**

---

## Why it matters

Terra Dourada is designed to preserve **semantic identity** instead of generating answers by probability alone.

Because recall is deterministic and auditable, the system can:
- preserve identity and conceptual consistency across long interactions
- avoid semantic drift and hallucinated meaning
- support structured study, research, and knowledge consolidation
- enable reproducible reasoning instead of probabilistic guessing
- operate in a sovereign way, without external dependency for recall

This makes Terra Dourada suitable for:
- study and learning systems
- research notes and knowledge bases
- identity-aware assistants
- environments that require transparency and auditability

---

## Project status

This project is experimental and under active development.

Thresholds, heuristics, and APIs may evolve, but the core principles of deterministic recall and semantic sovereignty are stable.

---

## License

MIT



## 📊 Results and Technical Report

See the [`report.txt`](./report.txt) file for a detailed technical report covering:
- evaluation criteria and metrics
- qualitative analysis of system behavior
- preliminary results
- architectural insights and design considerations

This report complements the README with deeper technical details that may be of interest to developers and evaluators.



---

## 🧠 Sovereign Training (mind.bin)

Terra Dourada provides a **public deterministic training service** responsible for generating auditable `mind.bin` files from raw text.

This training is **not a prompt** and **does not rely on generative models** to create memory.  
It produces a sovereign semantic base that can be loaded locally or accessed via API.

### 🔗 Public training endpoint
https://terra-dourada-gpt-green-butterfly-3484.fly.dev/

### 📦 What this service does
- Accepts raw text input
- Executes the deterministic training pipeline (FXL Turbo)
- Generates a `mind.bin` file
- Returns a `.zip` archive containing:
  - `mind.bin`
  - `report.txt` (training log)

### ⚠️ Important notes
- The service is **stateless** (each training job is isolated)
- The generated `mind.bin` belongs exclusively to the user who created it
- The training endpoint is exposed for **technical demonstration and validation**
- The internal training logic is part of the Terra Dourada core system
---
- ## 🟡 Terra Dourada — Fractal GPT

**Terra Dourada Fractal GPT** is a **deterministic artificial intelligence system** built on **explicit semantic memory**, with no weights, no opaque embeddings, and no probabilistic learning.

Knowledge is constructed through **canonical declarations** and controlled semantic variations, resulting in a fully auditable, reproducible, and verifiable model.

🔹 Not an LLM  
🔹 No global statistics  
🔹 No hallucinations  
🔹 Every response is traceable to memory (`mind.bin`)

🌐 **Public demo**:  
👉 https://terra-dourada-fractal-gpt.fly.dev/

## License

Copyright (c) 2025 Armando Freire (Terra Dourada)

All rights reserved.

This software and documentation are proprietary.
No permission is granted to use, copy, modify, distribute,
or sell this material without explicit written consent.




