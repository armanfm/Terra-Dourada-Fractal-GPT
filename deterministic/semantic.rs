#[derive(Debug, Clone)]
pub struct SemanticEntry {
    pub id: u64,
    pub texto: String,
}

#[derive(Debug)]
pub struct SemanticMemory {
    pub banco: Vec<SemanticEntry>,
    pub proximo_id: u64,
}

impl SemanticMemory {
    pub fn new() -> Self {
        Self {
            banco: Vec::new(),
            proximo_id: 0,
        }
    }

    // Adiciona qualquer texto sem checagem ou busca
    pub fn learn(&mut self, texto: &str) {
        let entry = SemanticEntry {
            id: self.proximo_id,
            texto: texto.to_string(),
        };
        self.banco.push(entry);
        self.proximo_id += 1;
    }
}

    }

    text
}

