use std::env;
use std::fs;

fn normalize_token(s: &str) -> String {
    s.chars()
        .filter(|c| c.is_alphanumeric() || *c == '-' || *c == '_')
        .flat_map(|c| c.to_lowercase())
        .collect::<String>()
}

fn tokens_ordered_unique(s: &str) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    let mut seen: std::collections::HashSet<String> = std::collections::HashSet::new();

    for raw in s.split_whitespace() {
        let t = normalize_token(raw);
        if t.len() < 2 {
            continue;
        }
        if seen.insert(t.clone()) {
            out.push(t);
        }
    }
    out
}

fn split_sentences(text: &str) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    let mut buf = String::new();

    for ch in text.chars() {
        buf.push(ch);
        if ch == '.' || ch == '?' || ch == '!' || ch == '\n' {
            let s = buf.trim().to_string();
            if !s.is_empty() {
                out.push(s);
            }
            buf.clear();
        }
    }

    let tail = buf.trim().to_string();
    if !tail.is_empty() {
        out.push(tail);
    }
    out
}

fn alpha_count(s: &str) -> usize {
    s.chars().filter(|c| c.is_alphabetic()).count()
}

fn load_mind_minimal(path: &str) -> Vec<String> {
    let raw = match fs::read(path) {
        Ok(b) => b,
        Err(_) => return Vec::new(),
    };

    let texto = String::from_utf8_lossy(&raw)
        .replace('\0', " ")
        .replace('\u{FFFD}', " ");

    let mut banco: Vec<String> = Vec::new();
    let mut buffer = String::new();

    for ch in texto.chars() {
        buffer.push(ch);

        // delimitadores naturais (igual teu loader)
        if ch == '.' || ch == '?' || ch == '!' || ch == '\n' || buffer.len() >= 400 {
            let s = buffer.trim().to_string();
            buffer.clear();

            // filtros simples (igual teu loader)
            if s.len() < 25 {
                continue;
            }
            if alpha_count(&s) < 12 {
                continue;
            }

            banco.push(s);
        }
    }

    banco
}

fn overlap_score(query_tokens: &[String], cand: &str) -> i32 {
    if query_tokens.is_empty() {
        return 0;
    }
    let cand_tokens = tokens_ordered_unique(cand);
    if cand_tokens.is_empty() {
        return 0;
    }

    let cand_set: std::collections::HashSet<&str> = cand_tokens.iter().map(|x| x.as_str()).collect();

    let mut score = 0i32;
    for t in query_tokens {
        if cand_set.contains(t.as_str()) {
            score += 1;
        }
    }
    score
}

fn main() {
    // argv[1] = pergunta
    let query = env::args().nth(1).unwrap_or_default();
    let query = query.trim().to_string();

    if query.is_empty() {
        // Go vai tratar como recall vazio/sem utilidade
        return;
    }

    // ENV vindo do Go: TD_MIND_PATH=/tmp/td_mind_xxx.bin
    let mind_path = match env::var("TD_MIND_PATH") {
        Ok(v) => v,
        Err(_) => {
            println!("(mind não carregado)");
            return;
        }
    };

    let banco = load_mind_minimal(&mind_path);
    if banco.is_empty() {
        println!("(não sei)");
        return;
    }

    // Busca determinística por overlap de tokens
    let qt = tokens_ordered_unique(&query);

    let mut scored: Vec<(i32, usize)> = Vec::new(); // (score, idx)
    for (i, s) in banco.iter().enumerate() {
        let sc = overlap_score(&qt, s);
        if sc > 0 {
            scored.push((sc, i));
        }
    }

    // Ordena: maior score primeiro; empate: menor idx (mais antigo) primeiro -> determinístico
    scored.sort_by(|a, b| b.0.cmp(&a.0).then_with(|| a.1.cmp(&b.1)));

    if scored.is_empty() {
        // fallback mínimo (igual tua ideia: último)
        println!("{}", banco.last().unwrap());
        return;
    }

    // imprime top N linhas (stdout = recall)
    let n = scored.len().min(8);
    for k in 0..n {
        let idx = scored[k].1;
        println!("{}", banco[idx]);
    }
}
