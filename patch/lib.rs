// ======================================================
// 📦 DECLARAÇÃO DE MÓDULOS DO CRATE
// ======================================================

// ----------------------
// Núcleo semântico
// ----------------------
pub mod semantic;
pub mod learning;
pub mod persistencia;

// ----------------------
// Runtime
// ----------------------
pub mod fxl_runtime;

// ----------------------
// Núcleo FXL (base)
// ----------------------


// ----------------------
// Extensões FXL
// ----------------------


// ======================================================
// 🔁 REEXPORTS (API PÚBLICA DO CRATE)
// ======================================================

// ----------------------
// Semantic core (mínimo real)
// ----------------------
pub use semantic::SemanticMemory;

// ----------------------
// Learning / persistência
// ----------------------
pub use learning::load_learning_file;
pub use persistencia::load_binary;

// ----------------------
// Runtime
// ----------------------
pub use fxl_runtime::FxlRuntime;
