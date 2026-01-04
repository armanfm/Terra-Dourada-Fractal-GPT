use std::env;
use std::fs;
use std::collections::HashSet;

fn normalize_token(s: &str) -> String {
    s.chars()
        .filter(|c| c.is_alphanumeric() || *c == '-' || *c == '_')
        .flat_map(|c| c.to_lowercase())
        .collect()
}

fn tokens_ordered_unique(s: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut seen = HashSet::new();

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

fn alpha_count(s: &str) -> usize {
    s.chars().filter(|c| c.is_alphabetic()).count()
}

/// Extrai SOMENTE strings ASCII legíveis do binário
fn extract_ascii_strings(raw: &[u8]) -> Vec<String> {
    let mut out = Vec::new();
    let mut buf = Vec::new();

    for &b in raw {
        if b.is_ascii_graphic() || b == b' ' {
            buf.push(b);
        } else {
            if buf.len() >= 20 {
                if let Ok(s) = String::from_utf8(buf.clone()) {
                    if alpha_count(&s) >= 10 {
                        out.push(s.trim().to_string());
                    }
                }
            }
            buf.clear();
        }
    }

    if buf.len() >= 20 {
        if let Ok(s) = String::from_utf8(buf) {
            if alpha_count(&s) >= 10 {
                out.push(s.trim().to_string());
            }
        }
    }

    out
}

fn overlap_score(query_tokens: &[String], cand: &str) -> i32 {
    if query_tokens.is_empty() {
        return 0;
    }

    let cand_tokens = tokens_ordered_unique(cand);
    if cand_tokens.is_empty() {
        return 0;
    }

    let cand_set: HashSet<&str> =
        cand_tokens.iter().map(|x| x.as_str()).collect();

    let mut score = 0;
    for t in query_tokens {
        if cand_set.contains(t.as_str()) {
            score += 1;
        }
    }
    score
}

fn main() {
    // argv[1] = query vinda do Go
    let query = env::args().nth(1).unwrap_or_default();
    let query = query.trim();
    if query.is_empty() {
        return;
    }

    let mind_path = match env::var("TD_MIND_PATH") {
        Ok(v) => v,
        Err(_) => return,
    };

    let raw = match fs::read(&mind_path) {
        Ok(b) => b,
        Err(_) => return,
    };

    let banco = extract_ascii_strings(&raw);
    if banco.is_empty() {
        return;
    }

    let qt = tokens_ordered_unique(query);

    let mut scored: Vec<(i32, usize)> = Vec::new();
    for (i, s) in banco.iter().enumerate() {
        let sc = overlap_score(&qt, s);
        if sc > 0 {
            scored.push((sc, i));
        }
    }

    scored.sort_by(|a, b| b.0.cmp(&a.0).then_with(|| a.1.cmp(&b.1)));

    if scored.is_empty() {
        return;
    }

    let n = scored.len().min(8);
    for k in 0..n {
        println!("{}", banco[scored[k].1]);
    }
}
