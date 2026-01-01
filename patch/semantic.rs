pub struct SemanticMemory {
    pub banco: Vec<String>,
}

impl SemanticMemory {
    pub fn new() -> Self {
        Self {
            banco: Vec::new(),
        }
    }

    pub fn learn(&mut self, texto: String) {
        self.banco.push(texto);
    }
}
