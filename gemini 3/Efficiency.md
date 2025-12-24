## Token Efficiency and Cost Control

Terra Dourada was designed to minimize LLM token usage by separating semantic memory from text generation.

Instead of simulating memory by resending long conversation histories or documents within the LLM context window, Terra Dourada performs deterministic semantic recall locally and injects only the minimal relevant fragment into the Gemini 3 prompt.

As a result, token usage remains low, predictable, and independent of the size of the memory or the length of prior interactions.

This design directly reduces operational costs, latency, and semantic drift, while preserving contextual accuracy.

---

## Architecture: Memory Outside the Context Window

Traditional LLM-based systems emulate memory by repeatedly sending prior messages and documents within the model’s context window. As interactions grow, this approach increases token consumption, cost, latency, and the risk of semantic degradation.

Terra Dourada uses a different architecture:

- Semantic memory is stored in a deterministic binary format (`mind.bin`)
- Recall is computed locally using a deterministic recall engine
- Only the selected recall fragment is injected into the Gemini 3 prompt
- The LLM is never responsible for storing or retrieving memory

This design ensures that Gemini 3 is used strictly as a **language synthesis layer**, while memory, identity, and continuity are handled externally in a transparent and auditable way.

---

## Empirical Token Usage

Gemini API logs confirm the effectiveness of this approach in real usage:

- Typical input tokens: ~250  
- Typical output tokens: ~80  
- Total tokens per request: under 800  

This token usage remains stable regardless of the size of the semantic memory (`mind.bin`) or the number of prior interactions.

The dialogue examples shown in this project were generated using this architecture, where deterministic recall provides the content and Gemini 3 is used solely for linguistic rendering.


<img width="1920" height="1080" alt="image" src="https://github.com/user-attachments/assets/62acc313-e9da-4e12-acbd-804f8c8da100" />




