use std::fs::File;
use std::io::{BufRead, BufReader};

use crate::semantic::SemanticMemory;

pub fn load_learning_file(memory: &mut SemanticMemory, path: &str) {
    let file = File::open(path)
        .expect("não consegui abrir arquivo de aprendizado");
    let reader = BufReader::new(file);

    for line in reader.lines() {
        let line = line.expect("erro lendo linha");

        // ignorar linhas vazias e comentários
        if line.trim().is_empty() || line.starts_with('#') {
            continue;
        }

        // ❌ NÃO EXISTE MAIS learn
        // ❌ NÃO FAZ NADA AQUI
        // aprendizado foi removido desse estágio

        let _ = &memory; // evita warning
    }
}
