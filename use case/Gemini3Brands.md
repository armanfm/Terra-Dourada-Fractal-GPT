## Use of Gemini in Terra Dourada Brands

### Exclusive role of Gemini

In Terra Dourada Brands, **Gemini is used exclusively as a semantic review and explanation layer**.  
It **does not participate** in search, calculation, ranking, or technical decision-making.

Gemini’s role is to **semantically interpret an already determined result** and transform it into a **human-readable analysis**.

---

### Input provided to Gemini

Gemini receives only:

- the **queried name**
- the **best identified candidate**
- **precomputed technical metrics**
- **minimal segment context** (when provided)

Gemini **does not receive databases**, **does not receive full lists**, and **does not perform algorithmic comparison**.

---

### What Gemini analyzes

Based on these inputs, Gemini performs a **controlled semantic review**, addressing:

- **Phonetics**  
  Evaluation of how the names sound when spoken and the risk of auditory confusion.

- **Visual nominative aspects**  
  Word structure, roots, textual composition, and nominative visual perception.

- **Conceptual overlap**  
  Shared ideas, implicit meanings, and symbolic proximity between the terms.

- **Human perception of confusion risk**  
  How an average consumer might interpret or confuse the names in the market.

---

### Output produced by Gemini

Gemini produces exclusively:

- a **structured textual explanation**
- a **semantic recommendation** (e.g., high risk, gray zone, lower risk)
- a **discursive confidence estimate**, based on the completeness of the presented arguments

This output:

- **has no legal value**
- **does not replace human decision-making**
- **does not alter the technical result**

---

### Control via parameters (tokens)

The **max output tokens** parameter controls **only the depth of the explanation**:

- lower limits → more concise explanations  
- higher limits → more detailed and well-founded explanations  

Increasing the token limit **does not change the conclusion**; it only **expands the level of detail in the semantic analysis**.

---

### What Gemini does NOT do

- ❌ does not calculate similarity  
- ❌ does not choose candidates  
- ❌ does not make legal approval or rejection decisions  
- ❌ does not access external data  
- ❌ does not alter technical results  

---

### Gemini’s role in the workflow

Gemini operates **after** technical processing and **before** final human decision-making, acting as:

> **a bridge between deterministic technical metrics and human understanding.**

It converts technical evidence into **clear language**, supporting analysis, oversight, and informed review.

---

### Justification for using Gemini

Gemini is fundamental to Terra Dourada Brands because it:

- has strong **structured semantic analysis** capabilities
- translates technical signals into **understandable arguments**
- reduces human interpretation time
- keeps the final decision **outside the AI**

---

### Summary

- Gemini is **central**, but **strictly bounded**
- It acts as a **semantic reviewer and explainer**
- It does not interfere with calculation or technical truth
- It scales the **quality of explanation**, not the outcome
