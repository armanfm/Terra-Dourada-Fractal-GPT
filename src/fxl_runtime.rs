use crate::persistencia::load_binary;
use crate::semantic::{SemanticMemory, recall};

pub struct FxlRuntime {
    pub memory: SemanticMemory,
}

impl FxlRuntime {
    pub fn load(path: &str) -> Self {
        let memory = load_binary(path);
        Self { memory }
    }

    pub fn ask(&self, query: &str) -> String {
        if let Some(entry) = recall(&self.memory, query) {
            entry.texto.clone()
        } else {
            "(não sei)".to_string()
        }
    }
}
