use std::collections::{HashMap, HashSet};
use sha2::{Sha256, Digest};
use base64::{Engine as _, engine::general_purpose::STANDARD as BASE64};


const LIMIAR_SIMILARIDADE: f64 = 0.75;
const LIMIAR_SCORE_FINAL: f64 = 0.40;

// Funil estrutural minucioso (sem pesos)
const BITS_ESTRUTURAL: usize = 512;

// Peso do est (FOCO) no score final
const PESO_ESTRUTURAL: f64 = 0.10;

// Peso do est GLOBAL no score final (bem leve)
const PESO_ESTRUTURAL_GLOBAL: f64 = 0.05;

// Guilhotinas
const LIMIAR_ESTRUTURAL: f64 = 0.55;         // FOCO
const LIMIAR_ESTRUTURAL_GLOBAL: f64 = 0.55;  // GLOBAL

// TOP-K final (quantas respostas retornar)
const TOP_K_PADRAO: usize = 10;
const EPS_SCORE: f64 = 1e-12;

// Anti-redundância (Jaccard em tokens informacionais)
const LIMIAR_REDUNDANCIA_TOPK: f64 = 0.55;


const LIMIAR_JACCARD_PREFILTRO: f64 = 0.12;      
const LIMIAR_BYTES_PREFILTRO: f64 = 0.55;       
const LIMIAR_JACCARD_PARA_TOKENS: f64 = 0.14;    
const TOPK_BRUTO_MULT: usize = 6;                
const TOPK_BRUTO_MAX: usize = 64;                


pub fn bytes_from_text(texto: &str) -> Vec<u8> {
    texto.as_bytes().to_vec()
}



#[derive(Debug, Clone)]
pub struct SemanticEntry {
    pub id: u64,
    pub texto: String,
    pub bytes_fx: Vec<u8>,
    pub hash_fx: u64,
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



pub fn soberano_hash(bytes: &[u8]) -> u64 {
    let mut h: u64 = 0xcbf29ce484222325;
    for &b in bytes {
        h ^= b as u64;
        h = h.wrapping_mul(0x100000001b3);
    }
    h
}


pub fn learn(memory: &mut SemanticMemory, texto: &str) {
    let bytes = bytes_from_text(texto);
    let hash = soberano_hash(&bytes);

    if memory.indice.contains_key(&hash) {
        return;
    }

    let entry = SemanticEntry {
        id: memory.proximo_id,
        texto: texto.to_string(),
        bytes_fx: bytes.clone(),
        hash_fx: hash,
    };

    memory.indice.insert(hash, memory.banco.len());
    memory.banco.push(entry);
    memory.proximo_id += 1;
}



fn fold_pt_char(c: char) -> char {
    match c {
        'á' | 'à' | 'ã' | 'â' => 'a',
        'é' | 'ê' => 'e',
        'í' => 'i',
        'ó' | 'ô' | 'õ' => 'o',
        'ú' => 'u',
        'ç' => 'c',
        _ => c,
    }
}

fn norm_text(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut last_space = true;

    for ch in text.chars() {
        let lower = ch.to_lowercase().next().unwrap_or(ch);
        let f = fold_pt_char(lower);

        if f.is_ascii_alphanumeric() {
            out.push(f);
            last_space = false;
        } else if !last_space {
            out.push(' ');
            last_space = true;
        }
    }

    out.trim().to_string()
}

fn norm_token(tok: &str) -> String {
    norm_text(tok).replace(' ', "")
}

// siglas curtas (ex: "btu") viram foco real.
fn extrair_foco(query: &str) -> (String, Vec<String>, String) {
    let qn = norm_text(query);

    let mut anchors: Vec<String> = Vec::new();

    for raw in qn.split_whitespace() {
        let t = norm_token(raw);
        if t.is_empty() {
            continue;
        }

        let p = peso_token(&t);

        let sigla = (2..=4).contains(&t.len())
            && t.chars().all(|c| c.is_ascii_alphabetic())
            && p >= 0.99;

        let informacional = (p >= 0.99) && t.len() >= 4;
        let forte = (p >= 0.30) && t.len() >= 6;

        if sigla || informacional || forte {
            if !anchors.contains(&t) {
                anchors.push(t);
            }
        }
    }

    let tem_outro = anchors.iter().any(|t| t != "terra" && t != "dourada");
    if tem_outro {
        anchors.retain(|t| t != "terra" && t != "dourada");
    }

    let foco = if anchors.is_empty() { qn.clone() } else { anchors.join(" ") };

    (foco, anchors, qn)
}



fn string_para_bits(texto: &str, max_bits: usize) -> Vec<u8> {
    let mut bits = Vec::new();
    for byte in texto.as_bytes() {
        for i in (0..8).rev() {
            bits.push(((byte >> i) & 1) as u8);
        }
    }
    bits.truncate(max_bits);
    while bits.len() < max_bits {
        bits.push(0);
    }
    bits
}

fn hash_para_bits(hash: &[u8], max_bits: usize) -> Vec<u8> {
    let mut bits = Vec::new();
    for &byte in hash {
        for i in (0..8).rev() {
            bits.push(((byte >> i) & 1) as u8);
        }
    }
    bits.truncate(max_bits);
    while bits.len() < max_bits {
        bits.push(0);
    }
    bits
}

fn similaridade_bits(a: &[u8], b: &[u8]) -> f64 {
    let k = a.len().min(b.len());
    if k == 0 { return 0.0; }
    let iguais = (0..k).filter(|&i| a[i] == b[i]).count();
    iguais as f64 / k as f64
}



fn similaridade_bytes(a: &str, b: &str) -> f64 {
    let ba = string_para_bits(a, 128);
    let bb = string_para_bits(b, 128);
    similaridade_bits(&ba, &bb)
}

fn similaridade_sha256(a: &str, b: &str) -> f64 {
    let ha = Sha256::digest(a.as_bytes());
    let hb = Sha256::digest(b.as_bytes());

    let ba = hash_para_bits(&ha, 128);
    let bb = hash_para_bits(&hb, 128);

    similaridade_bits(&ba, &bb)
}

fn similaridade_base64(a: &str, b: &str) -> f64 {
    let a64 = BASE64.encode(a.as_bytes());
    let b64 = BASE64.encode(b.as_bytes());

    let ba = string_para_bits(&a64, 128);
    let bb = string_para_bits(&b64, 128);

    similaridade_bits(&ba, &bb)
}



fn similaridade_robusta(a: &str, b: &str) -> f64 {
    let s_bytes = similaridade_bytes(a, b);
    let s_sha = similaridade_sha256(a, b);

    if (s_bytes - s_sha).abs() < 0.15 {
        (s_bytes * 0.5) + (s_sha * 0.5)
    } else {
        let s_b64 = similaridade_base64(a, b);
        (s_bytes * 0.35) + (s_sha * 0.35) + (s_b64 * 0.30)
    }
}



fn similaridade_estrutural_minuciosa(a: &str, b: &str) -> (f64, f64, f64, f64) {
    let sb = similaridade_bits(
        &string_para_bits(a, BITS_ESTRUTURAL),
        &string_para_bits(b, BITS_ESTRUTURAL),
    );

    let ha = Sha256::digest(a.as_bytes());
    let hb = Sha256::digest(b.as_bytes());
    let ss = similaridade_bits(
        &hash_para_bits(&ha, BITS_ESTRUTURAL),
        &hash_para_bits(&hb, BITS_ESTRUTURAL),
    );

    let a64 = BASE64.encode(a.as_bytes());
    let b64 = BASE64.encode(b.as_bytes());
    let s64 = similaridade_bits(
        &string_para_bits(&a64, BITS_ESTRUTURAL),
        &string_para_bits(&b64, BITS_ESTRUTURAL),
    );

    let combinada = (sb * 0.3) + (ss * 0.3) + (s64 * 0.4);
    (sb, ss, s64, combinada)
}



fn peso_token(token: &str) -> f64 {
    let t = token.to_lowercase();

    match t.as_str() {
        "o" | "a" | "os" | "as" => 0.10,
        "um" | "uma" | "uns" | "umas" => 0.12,

        "de" | "do" | "da" | "dos" | "das" |
        "em" | "no" | "na" | "nos" | "nas" |
        "por" | "pelo" | "pela" | "pelos" | "pelas" |
        "para" | "pra" | "pro" |
        "com" | "sem" | "sob" | "sobre" |
        "ate" | "até" | "contra" | "entre" | "desde" => 0.15,

        "eu" | "tu" | "ele" | "ela" | "nós" | "vos" | "vós" | "eles" | "elas" |
        "voce" | "você" | "voces" | "vocês" |
        "me" | "te" | "se" | "lhe" | "lhes" |
        "mim" | "ti" | "si" => 0.15,

        "meu" | "minha" | "meus" | "minhas" |
        "seu" | "sua" | "seus" | "suas" |
        "nosso" | "nossa" | "nossos" | "nossas" |
        "dele" | "dela" | "deles" | "delas" => 0.18,

        "este" | "esta" | "estes" | "estas" | "isto" |
        "esse" | "essa" | "esses" | "essas" | "isso" |
        "aquele" | "aquela" | "aqueles" | "aquelas" | "aquilo" |
        "que" | "quem" | "qual" | "quais" |
        "onde" | "quando" | "como" |
        "quanto" | "quantos" | "quantas" |
        "porque" | "porquê" => 0.20,

        "algum" | "alguma" | "alguns" | "algumas" |
        "nenhum" | "nenhuma" |
        "todo" | "toda" | "todos" | "todas" |
        "outro" | "outra" | "outros" | "outras" |
        "alguem" | "alguém" | "ninguem" | "ninguém" | "nada" | "tudo" => 0.25,

        "e" | "ou" | "mas" | "porem" | "porém" | "contudo" |
        "logo" | "portanto" | "assim" | "nem" => 0.15,

        "é" | "sou" | "es" | "és" | "sao" | "são" |
        "era" | "eram" |
        "ser" | "estar" | "ter" | "haver" |
        "foi" | "esta" | "está" | "tem" | "ha" | "há" => 0.20,

        "nao" | "não" | "sim" | "tambem" | "também" | "ja" | "já" | "ainda" |
        "muito" | "pouco" | "bem" | "mal" |
        "aqui" | "ali" | "la" | "lá" |
        "hoje" | "ontem" | "amanha" | "amanhã" |
        "agora" | "depois" | "antes" => 0.25,

        _ if t.chars().all(|c| c.is_numeric() || c == ',' || c == '.') => 0.30,
        _ => 1.0,
    }
}



fn xor_frase_score(a: &str, b: &str) -> f64 {
    let ta: Vec<&str> = a.split_whitespace().collect();
    let tb: Vec<&str> = b.split_whitespace().collect();

    if ta.is_empty() || tb.is_empty() {
        return 0.0;
    }

    let mut soma: f64 = 0.0;
    let mut peso_total: f64 = 0.0;

    for wa in &ta {
        let peso = peso_token(wa);

        let mut melhor: f64 = 0.0;
        for wb in &tb {
            let s: f64 = similaridade_robusta(wa, wb);
            if s > melhor {
                melhor = s;
            }
        }

        soma += melhor * peso;
        peso_total += peso;
    }

    if peso_total == 0.0 { 0.0 } else { soma / peso_total }
}



fn penalidade_baixa_similaridade(sim: f64) -> f64 {
    if sim >= LIMIAR_SIMILARIDADE {
        0.0
    } else {
        let d = LIMIAR_SIMILARIDADE - sim;
        (d * d * 3.0).min(0.9)
    }
}



fn bonus_tipo_frase(texto: &str) -> f64 {
    let t = texto.to_lowercase();

    if t.contains(" não é ") || t.contains(" nao e ") {
        -0.08
    } else if t.contains(" é ") || t.contains(" e ") {
        0.12
    } else {
        0.0
    }
}


fn melhor_que(a_score: f64, a_id: u64, b_score: f64, b_id: u64) -> bool {
    if a_score > b_score + EPS_SCORE {
        true
    } else if (a_score - b_score).abs() <= EPS_SCORE && a_id < b_id {
        true
    } else {
        false
    }
}

fn inserir_top_k<'a>(
    top: &mut Vec<(f64, u64, &'a SemanticEntry)>,
    score: f64,
    entry: &'a SemanticEntry,
    k: usize,
) {
    let id = entry.id;

    let mut pos = 0usize;
    while pos < top.len() {
        let (s, eid, _) = top[pos];
        if melhor_que(score, id, s, eid) {
            break;
        }
        pos += 1;
    }

    if pos < k {
        top.insert(pos, (score, id, entry));
        if top.len() > k {
            top.pop();
        }
    }
}


fn tokens_informacionais_set(texto: &str) -> HashSet<String> {
    let n = norm_text(texto);
    let mut set = HashSet::new();

    for raw in n.split_whitespace() {
        let p = peso_token(raw);
        if p >= 0.99 && raw.len() >= 3 {
            set.insert(raw.to_string());
        }
    }
    set
}

fn jaccard_set(a: &HashSet<String>, b: &HashSet<String>) -> f64 {
    if a.is_empty() || b.is_empty() {
        return 0.0;
    }

    let (small, large) = if a.len() <= b.len() { (a, b) } else { (b, a) };

    let mut inter: usize = 0;
    for t in small.iter() {
        if large.contains(t) {
            inter += 1;
        }
    }

    let uni = a.len() + b.len() - inter;
    if uni == 0 { 0.0 } else { inter as f64 / uni as f64 }
}



fn eh_redundante_set(te: &HashSet<String>, escolhidos: &[(u64, HashSet<String>)]) -> bool {
    for (_id, t2) in escolhidos.iter() {
        let jac = jaccard_set(te, t2);
        if jac >= LIMIAR_REDUNDANCIA_TOPK {
            return true;
        }
    }
    false
}



pub fn recall_top_k<'a>(
    memory: &'a SemanticMemory,
    query: &str,
    k: usize,
) -> Vec<&'a SemanticEntry> {
    let k_final = k.max(1);
    let k_bruto = (k_final * TOPK_BRUTO_MULT).min(TOPK_BRUTO_MAX);

    let qbytes = bytes_from_text(query);
    let (foco, anchors, qn) = extrair_foco(query);

    // pré-cálculos da query
    let q_info = tokens_informacionais_set(&qn);

    // coletor bruto por score
    let mut top: Vec<(f64, u64, &SemanticEntry)> = Vec::with_capacity(k_bruto);

    for entry in &memory.banco {
        // 0) delta_len (grátis)
        let delta_len = qbytes.len().abs_diff(entry.bytes_fx.len());
        if delta_len > 300 {
            continue;
        }

        // normalização do entry (barato, e é base pra âncoras e jaccard)
        let en = norm_text(&entry.texto);

        // 1) âncoras (grátis)
        if !anchors.is_empty() {
            let mut tem = false;
            for a in &anchors {
                if en.split_whitespace().any(|t| t == a) {
                    tem = true;
                    break;
                }
            }
            if !tem {
                continue;
            }
        }

        // 2) jaccard barato (mata lixo cedo)
        let e_info = if q_info.is_empty() {
            HashSet::new()
        } else {
            tokens_informacionais_set(&en)
        };

        let jac_pref = if q_info.is_empty() { 1.0 } else { jaccard_set(&q_info, &e_info) };
        if !q_info.is_empty() && jac_pref < LIMIAR_JACCARD_PREFILTRO {
            continue;
        }

        // 3) bytes rápido (antes de SHA/Base64 pesado)
        let sim_b = similaridade_bytes(query, &entry.texto);
        if sim_b < LIMIAR_BYTES_PREFILTRO && jac_pref < (LIMIAR_JACCARD_PREFILTRO + 0.08) {
            continue;
        }

        // 4) robusto (pode chamar SHA/Base64)
        let sim_global = similaridade_robusta(query, &entry.texto);

        // 5) token-score (o mais caro): só roda se fizer sentido
        let sim_tokens = if sim_global >= LIMIAR_SIMILARIDADE || jac_pref >= LIMIAR_JACCARD_PARA_TOKENS {
            xor_frase_score(query, &entry.texto)
        } else {
            0.0
        };

        // mesma regra de corte (só que agora economiza cálculo caro quando jac_pref é baixo)
        if sim_global < LIMIAR_SIMILARIDADE && sim_tokens < LIMIAR_SIMILARIDADE {
            continue;
        }

        // 6) funil estrutural GLOBAL (caro) — só depois que passou no barato
        let (_gsb, _gss, _gs64, est_global) = similaridade_estrutural_minuciosa(&qn, &en);
        if est_global < LIMIAR_ESTRUTURAL_GLOBAL {
            continue;
        }

        // 7) funil estrutural FOCO (caro)
        let (_sb, _ss, _s64, est_foco) = similaridade_estrutural_minuciosa(&foco, &en);
        if est_foco < LIMIAR_ESTRUTURAL {
            continue;
        }

        // score
        let pen_token = penalidade_baixa_similaridade(sim_tokens);
        let pen_global = penalidade_baixa_similaridade(sim_global);
        let pen_len = delta_len as f64 / 800.0;

        let score_base =
              (sim_tokens * 0.7)
            + (sim_global * 0.3)
            + (est_foco * PESO_ESTRUTURAL)
            + (est_global * PESO_ESTRUTURAL_GLOBAL);

        let mut score =
              score_base
            - pen_token
            - pen_global
            - pen_len;

        score += bonus_tipo_frase(&entry.texto);

        inserir_top_k(&mut top, score, entry, k_bruto);
    }

    if top.is_empty() || top[0].0 < LIMIAR_SCORE_FINAL {
        return Vec::new();
    }

    // ✅ anti-redundância em cima do top ordenado
    let mut escolhidos: Vec<&SemanticEntry> = Vec::with_capacity(k_final);

    // cache de tokens informacionais dos escolhidos
    let mut escolhidos_tokens: Vec<(u64, HashSet<String>)> = Vec::with_capacity(k_final);

    for (score, _id, entry) in top {
        if score < LIMIAR_SCORE_FINAL {
            continue;
        }
        if escolhidos.len() >= k_final {
            break;
        }

        let te = tokens_informacionais_set(&entry.texto);

        if eh_redundante_set(&te, &escolhidos_tokens) {
            continue;
        }

        escolhidos.push(entry);
        escolhidos_tokens.push((entry.id, te));
    }

    escolhidos
}



pub fn recall<'a>(
    memory: &'a SemanticMemory,
    query: &str,
) -> Option<&'a SemanticEntry> {
    recall_top_k(memory, query, 1).into_iter().next()
}



pub fn respond(memory: &SemanticMemory, input: &str) -> String {
    let top = recall_top_k(memory, input, TOP_K_PADRAO);

    if top.is_empty() {
        return "(não sei)".to_string();
    }

    let mut out = String::new();
    for (i, e) in top.iter().enumerate() {
        if i > 0 {
            out.push('\n');
        }
        out.push_str(&e.texto);
    }
    out
}

