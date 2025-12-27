use std::collections::HashMap;

// ======================================================
// CONFIG
// ======================================================

const TOP_K_PADRAO: usize = 10;
const MAX_AMPLIFY: usize = 3;

// ======================================================
// ESTRUTURAS
// ======================================================

#[derive(Debug, Clone)]
pub struct SemanticEntry {
    pub id: u64,
    pub texto: String,
    pub hash: u64,
}

#[derive(Debug)]
pub struct SemanticMemory {
    pub banco: Vec<SemanticEntry>,
    pub indice: HashMap<u64, usize>,
    pub proximo_id: u64,
}

impl SemanticMemory {
    pub fn new() -> Self {
        Self {
            banco: Vec::new(),
            indice: HashMap::new(),
            proximo_id: 0,
        }
    }
}

// ======================================================
// HASH SOBERANO (FNV-1A)
// ======================================================

pub fn soberano_hash(bytes: &[u8]) -> u64 {
    let mut h: u64 = 0xcbf29ce484222325;
    for &b in bytes {
        h ^= b as u64;
        h = h.wrapping_mul(0x100000001b3);
    }
    h
}

// ======================================================
// LEARN (DEDUPE POR HASH)
// ======================================================

pub fn learn(memory: &mut SemanticMemory, texto: &str) {
    let hash = soberano_hash(texto.as_bytes());

    if memory.indice.contains_key(&hash) {
        return;
    }

    let entry = SemanticEntry {
        id: memory.proximo_id,
        texto: texto.to_string(),
        hash,
    };

    memory.indice.insert(hash, memory.banco.len());
    memory.banco.push(entry);
    memory.proximo_id += 1;
}

// ======================================================
// RECALL GLOBAL (XOR)
// ======================================================

pub fn recall_top_k<'a>(
    memory: &'a SemanticMemory,
    query: &str,
    k: usize,
) -> Vec<&'a SemanticEntry> {
    let qh = soberano_hash(query.as_bytes());

    let mut scored: Vec<(u64, u64, &'a SemanticEntry)> = Vec::new();
    for e in &memory.banco {
        let dist = qh ^ e.hash;
        scored.push((dist, e.id, e));
    }

    scored.sort_by(|a, b| {
        if a.0 != b.0 {
            a.0.cmp(&b.0)
        } else {
            a.1.cmp(&b.1)
        }
    });

    scored
        .into_iter()
        .take(k.max(1))
        .map(|(_, _, e)| e)
        .collect()
}

// ======================================================
// DETECÇÃO DE CORRUPÇÃO
// ======================================================

fn has_interrogation(s: &str) -> bool {
    s.chars().any(|c| c == '?' || c == '�' || c.is_control())
}

// ======================================================
// SELEÇÃO DA MELHOR FALA COM INTERROGAÇÃO
// ======================================================

fn pick_best_corrupted_line<'a>(
    query: &str,
    lines: Vec<&'a str>,
) -> Option<&'a str> {
    let qh = soberano_hash(query.as_bytes());

    lines
        .into_iter()
        .map(|l| (qh ^ soberano_hash(l.as_bytes()), l))
        .min_by_key(|(d, _)| *d)
        .map(|(_, l)| l)
}

// ======================================================
// AMPLIFICAÇÃO LOCAL (recall dentro do recall)
// ======================================================

fn amplify_from_seed(seed: &str, query: &str) -> String {
    let qh = soberano_hash(query.as_bytes());

    let mut parts: Vec<(u64, &str)> = seed
        .lines()
        .map(|l| (qh ^ soberano_hash(l.as_bytes()), l))
        .collect();

    parts.sort_by_key(|(d, _)| *d);

    parts
        .into_iter()
        .map(|(_, l)| l)
        .collect::<Vec<_>>()
        .join("\n")
}

// ======================================================
// RESPOND FINAL (COM LOOP DE AMPLIFICAÇÃO)
// ======================================================

pub fn respond(memory: &SemanticMemory, query: &str) -> String {
    let top = recall_top_k(memory, query, TOP_K_PADRAO);

    let mut text = top
        .iter()
        .map(|e| e.texto.as_str())
        .collect::<Vec<_>>()
        .join("\n");

    for _ in 0..MAX_AMPLIFY {
        if !has_interrogation(&text) {
            break; // condição final atingida
        }

        let candidates: Vec<&str> = text
            .lines()
            .filter(|l| has_interrogation(l))
            .collect();

        if candidates.is_empty() {
            break;
        }

        if let Some(seed) = pick_best_corrupted_line(query, candidates) {
            text = amplify_from_seed(seed, query);
        } else {
            break;
        }
    }

    text
}
