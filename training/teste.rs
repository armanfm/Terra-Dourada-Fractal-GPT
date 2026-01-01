use sha2::{Sha256, Digest};
use base64::{Engine as _, engine::general_purpose::STANDARD as BASE64};

// =========================================================
// UTILITÁRIOS BÁSICOS
// =========================================================

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

fn similaridade_bits(a: &[u8], b: &[u8]) -> f64 {
    let k = a.len().min(b.len());
    if k == 0 { return 0.0; }

    let iguais = (0..k).filter(|&i| a[i] == b[i]).count();
    iguais as f64 / k as f64
}

// =========================================================
// MÉTODOS DE SIMILARIDADE
// =========================================================

fn similaridade_bytes(a: &str, b: &str) -> f64 {
    let ba = string_para_bits(a, 128);
    let bb = string_para_bits(b, 128);
    similaridade_bits(&ba, &bb)
}

fn similaridade_sha256(a: &str, b: &str) -> f64 {
    let ha = Sha256::digest(a.as_bytes());
    let hb = Sha256::digest(b.as_bytes());

    let ba = string_para_bits(&String::from_utf8_lossy(&ha), 128);
    let bb = string_para_bits(&String::from_utf8_lossy(&hb), 128);

    similaridade_bits(&ba, &bb)
}

fn similaridade_base64(a: &str, b: &str) -> f64 {
    let a64 = BASE64.encode(a.as_bytes());
    let b64 = BASE64.encode(b.as_bytes());

    let ba = string_para_bits(&a64, 128);
    let bb = string_para_bits(&b64, 128);

    similaridade_bits(&ba, &bb)
}

// =========================================================
// TESTE COMPARATIVO
// =========================================================

fn comparar(a: &str, b: &str) {
    let s_bytes = similaridade_bytes(a, b);
    let s_sha   = similaridade_sha256(a, b);
    let s_b64   = similaridade_base64(a, b);

    let combinada = (s_bytes * 0.3) + (s_sha * 0.3) + (s_b64 * 0.4);

    println!("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
    println!("A: '{}'", a);
    println!("B: '{}'", b);
    println!("Bytes : {:>6.2}%", s_bytes * 100.0);
    println!("SHA256: {:>6.2}%", s_sha * 100.0);
    println!("Base64: {:>6.2}%", s_b64 * 100.0);
    println!("▶ Comb.: {:>6.2}%", combinada * 100.0);
}

// =========================================================
// MAIN
// =========================================================

fn main() {
    println!("🔬 LABORATÓRIO BASE64 – ISOLADO E LIMPO");
    println!("=====================================");

    let casos = vec![
        ("terra dourada soberana", "terra dourada soberana"),
        ("terra dourada soberana", "terra dourada rainha"),
        ("gemini", "ethereum"),
        ("o cachorro corre no parque", "o cão corre no jardim"),
        ("gemini hackathon winner", "winner of the gemini hackathon"),
        ("gemini-hackathon-winner", "gemini hackathon winner"),
        ("gemini hackathon", "gemini hackathon"),
    ];

    for (a, b) in casos {
        comparar(a, b);
    }

    println!("=====================================");
    println!("✅ Experimento Base64 finalizado");
}

