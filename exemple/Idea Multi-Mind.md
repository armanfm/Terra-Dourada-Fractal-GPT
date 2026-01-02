## Modular RAG (Multi-Mind) — Deterministic Routing + Specialized `mind.bin` Files

### Overview
Terra Dourada supports a **Modular RAG** pattern: instead of a single giant knowledge base, the system maintains **multiple specialized `mind.bin` files** (topic-specific “minds”).  
A **deterministic router** (FXL Turbo) selects the correct mind for the user’s question *before* any LLM is prompted.

This is not “a bigger prompt.” It is **structured, modular knowledge** + **deterministic selection** + **minimal, controlled prompting**.

---

### Why multiple `mind.bin` files
A single monolithic knowledge base tends to:
- mix unrelated topics,
- increase retrieval noise,
- waste tokens,
- and raise the chance the LLM answers from the wrong context.

With Multi-Mind, each file is an **expert module**:
- `brasil_descobrimento.bin`
- `brasil_republica.bin`
- `regras_hackathon.bin`
- `empresa_rh.bin`
- `empresa_financeiro.bin`

Each mind is trained and maintained independently, so the system can **switch experts** deterministically.

---

### The Router (FXL Turbo) — how the right mind is selected
The router treats the user’s question as a signal and evaluates which mind is the best fit using:

- **Similarity score** (decision factor)
- **Context stability** (temporal consistency / anti-noise filter)
- **Rupture detection** (mismatch indicator)

**Key concept:** similarity is not “human semantics.” It is a **deterministic signal** that works extremely well as a *routing filter* and a *stability gate* inside your pipeline.

The router can operate in three practical modes:

1) **Single-Mind Load (default)**  
   Pick the top mind and load only it (fastest and cleanest).

2) **Top-K Load (when ambiguous)**  
   If scores are close, load 2–3 candidates and let deterministic retrieval pick the best excerpts.

3) **Mismatch Guard (rupture)**  
   If the loaded mind shows instability/rupture against the user’s question, the system blocks unsafe prompting and requests clarification (or switches mind).

---

### Retrieval stage — “stable excerpt” instead of raw dumps
After selecting the mind, recall does not send the whole binary to the LLM. Instead it produces a **stable excerpt**:

- short, topic-aligned fragments
- de-duplicated
- optionally ranked by similarity/context
- optionally accompanied by metadata (line index, section label, timestamp)

This is what makes it RAG in practice:
- deterministic selection  
- deterministic retrieval  
- minimal prompt payload

---

### Prompt stage — deterministic templates per domain
Each mind can have its own **prompt template** (also deterministic), such as:

- “Brazil History — factual answer, date-first, cite excerpt lines”
- “Hackathon Rules — eligibility + constraints + cite rule fragment”
- “Company HR — policy-only, no assumptions, cite policy ID”
- “Finance — numbers-only, no interpretation outside excerpt”

This prevents the LLM from behaving like a general chatbot and forces it to behave like a **verbalizer** of the retrieved truth.

---

### What this architecture gives you

#### 1) Lower hallucination pressure
The LLM receives **only the authorized, topic-correct excerpt**, not a giant mixed context.
If the wrong mind is selected or the question doesn’t match, **rupture detection** can stop the flow.

#### 2) Token efficiency
Instead of shipping “50 pages,” you ship the **minimal stable extract**.  
This is direct cost reduction and keeps answers tight.

#### 3) Data isolation & privacy boundaries
Different minds can represent different access scopes:
- HR vs Finance
- Public vs private
- Client A vs Client B

The LLM only sees what the router allows.

#### 4) Maintainability
Updating “Brazil Republic” does not touch “Discovery.”  
Each module can evolve independently.

#### 5) Observability (metrics that actually mean something)
Because routing and retrieval are deterministic, you can log:
- chosen mind
- similarity/context values
- rupture events
- excerpt size (tokens/bytes)
- throughput

This is the opposite of a black box.

---

### Practical example (Brazil history)
If the user asks:
> “What triggered the Proclamation of the Republic in Brazil?”

The router should select:
- `brasil_republica.bin` (not `brasil_descobrimento.bin`)

Then retrieval outputs a stable excerpt only about the Republic period.  
The LLM receives only that excerpt and must answer within it.

If the user asks:
> “What happened in 1500?”

The router selects:
- `brasil_descobrimento.bin`

Same pipeline. Different expert.

---

### README one-liner (if you want it short)
**“Terra Dourada supports Modular RAG via multiple `mind.bin` knowledge bases. FXL Turbo deterministically routes each query to the correct topic-mind (or blocks on mismatch) and prompts the LLM only with a minimal stable excerpt.”**

---

### Conclusion
Modular RAG (Multi-Mind) turns Terra Dourada into a **knowledge platform**, not just a chatbot:
- knowledge is separated into expert binaries,
- routing is deterministic and auditable,
- retrieval produces stable excerpts,
- the LLM is used only for fluent explanation—under strict constraints.

This architecture is exactly what makes the system scalable, cheap, and controllable.
