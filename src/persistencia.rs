use std::fs::File;
use std::io::Read;
use std::collections::HashMap;

use crate::semantic::{
    SemanticMemory,
    SemanticEntry,
    soberano_hash,
    bytes_from_text,
};

// ======================================================
// LOAD BINÁRIO — FX-TURBO / TERRAMIN REAL
// ======================================================

pub fn load_binary(path: &str) -> SemanticMemory {
    println!("🧠 [PERSIST][LOAD] FX-TURBO REAL MODE");
    println!("📂 [LOAD] arquivo: {}", path);

    let mut raw = Vec::new();
    if File::open(path).and_then(|mut f| f.read_to_end(&mut raw)).is_err() {
        println!("⚠️ [LOAD] arquivo não encontrado");
        return SemanticMemory::new();
    }

    println!(
        "🧪 [LOAD] assinatura bruta (16 bytes): {:?}",
        &raw[..raw.len().min(16)]
    );

    println!("📦 [LOAD] tamanho bruto = {} bytes", raw.len());

    let texto = String::from_utf8_lossy(&raw)
        .replace('\0', " ")
        .replace('\u{FFFD}', " ");

    let mut banco = Vec::new();
    let mut indice = HashMap::new();
    let mut buffer = String::new();

    let mut descartadas_curta = 0;
    let mut descartadas_alpha = 0;
    let mut descartadas_colisao = 0;

    for ch in texto.chars() {
        buffer.push(ch);

        // delimitadores naturais
        if matches!(ch, '.' | '?' | '!' | '\n') || buffer.len() >= 400 {
            let s = buffer.trim().to_string();
            buffer.clear();

            // filtros duros
            if s.len() < 25 {
                descartadas_curta += 1;
                continue;
            }

            if s.chars().filter(|c| c.is_alphabetic()).count() < 12 {
                descartadas_alpha += 1;
                continue;
            }

            let hash = soberano_hash(s.as_bytes());
            if indice.contains_key(&hash) {
                descartadas_colisao += 1;
                continue;
            }

            let id = banco.len() as u64;
            let bytes_fx = bytes_from_text(&s);

            //println!("----------------------------------------");
            //println!("🧠 [LOAD] ENTRY {}", id);
            //println!("TEXTO   : {}", s);
            //println!("LEN     : {}", s.len());
            //println!("BYTES_FX: {:?}", bytes_fx);
            //println!("HASH_FX : {}", hash);

            banco.push(SemanticEntry {
                id,
                texto: s.clone(),
                bytes_fx,
                hash_fx: hash,
            });

            indice.insert(hash, banco.len() - 1);
        }
    }

    

    let proximo_id = banco.len() as u64;



    SemanticMemory {
        banco,
        indice,
        proximo_id,
    }
}
