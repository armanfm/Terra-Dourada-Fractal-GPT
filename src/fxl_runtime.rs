use crate::persistencia::load_binary;
use crate::semantic::SemanticMemory;

pub struct FxlRuntime {
    pub memory: SemanticMemory,
}

impl FxlRuntime {
    pub fn load(path: &str) -> Self {
        let memory = load_binary(path);
        Self { memory }
    }

    pub fn ask(&self, _query: &str) -> String {
        // versão mínima: retorna o último texto armazenado, ou "(não sei)"
        self.memory
            .banco
            .last()
            .cloned()
            .unwrap_or_else(|| "(não sei)".to_string())
    }
}
