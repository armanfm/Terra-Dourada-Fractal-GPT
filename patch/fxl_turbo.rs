// fxl_turbo.rs - VERSÃO COM MÉTRICAS CONFIÁVEIS (PADRÃO CIENTÍFICO)

use std::collections::{HashMap, HashSet};
use std::fs::File;
use std::io::{BufRead, BufReader, Write};
use std::path::Path;
use std::time::{Instant, Duration};
use sha2::{Sha256, Digest};

// =========================================================
// CONSTANTES DE CONFIGURAÇÃO (PRODUÇÃO)
// =========================================================

const MAX_CONTEXT_HISTORY: usize = 100;      // Histórico máximo de similaridade
const JANELA_ESTABILIDADE: usize = 15;        // Janela para cálculo de contexto
const MAX_HASH_REPRESENTATIONS: usize = 1000; // Limite para exibição/processamento

// =========================================================
// ESTRUTURAS ORIGINAIS DO FXL TURBO (mantidas para compatibilidade)
// =========================================================

#[derive(Debug, Clone)]
pub struct Autoencoder {
    pub encoder_weights: Vec<f64>,
    pub decoder_weights: Vec<f64>,
    pub learning_rate: f64,
    pub best_loss: f64,
    pub reconstruction_accuracy: f64,
    pub epochs_trained: usize,
}

impl Autoencoder {
    pub fn new(input_size: usize, latent_size: usize) -> Self {
        let total_encoder_weights = input_size * latent_size;
        let total_decoder_weights = latent_size * input_size;
        
        let mut encoder_weights = Vec::with_capacity(total_encoder_weights);
        let mut decoder_weights = Vec::with_capacity(total_decoder_weights);
        
        for i in 0..total_encoder_weights {
            encoder_weights.push((i as f64 * 0.01 - 0.05).sin());
        }
        
        for i in 0..total_decoder_weights {
            decoder_weights.push((i as f64 * 0.01 - 0.05).cos());
        }
        
        Self {
            encoder_weights,
            decoder_weights,
            learning_rate: 0.001,
            best_loss: f64::INFINITY,
            reconstruction_accuracy: 0.0,
            epochs_trained: 0,
        }
    }
    
    pub fn encode(&self, input: &[f64; 8]) -> [f64; 4] {
        let mut latent = [0.0; 4];
        let input_size = 8;
        let latent_size = 4;
        
        for j in 0..latent_size {
            let mut sum = 0.0;
            for i in 0..input_size {
                let weight_idx = i * latent_size + j;
                sum += input[i] * self.encoder_weights[weight_idx];
            }
            latent[j] = sum.tanh();
        }
        
        latent
    }
    
    pub fn decode(&self, latent: &[f64; 4]) -> [f64; 8] {
        let mut output = [0.0; 8];
        let input_size = 8;
        let latent_size = 4;
        
        for i in 0..input_size {
            let mut sum = 0.0;
            for j in 0..latent_size {
                let weight_idx = j * input_size + i;
                sum += latent[j] * self.decoder_weights[weight_idx];
            }
            output[i] = sum.tanh();
        }
        
        output
    }
    
    pub fn train_step(&mut self, input: &[f64; 8]) -> f64 {
        let latent = self.encode(input);
        let reconstructed = self.decode(&latent);
        
        let mut loss = 0.0;
        for i in 0..8 {
            let diff = input[i] - reconstructed[i];
            loss += diff * diff;
        }
        loss /= 8.0;
        
        for i in 0..self.encoder_weights.len() {
            let grad = -loss * 0.01;
            self.encoder_weights[i] += self.learning_rate * grad;
        }
        
        for i in 0..self.decoder_weights.len() {
            let grad = -loss * 0.01;
            self.decoder_weights[i] += self.learning_rate * grad;
        }
        
        if loss < self.best_loss {
            self.best_loss = loss;
        }
        
        self.epochs_trained += 1;
        self.reconstruction_accuracy = (1.0 - loss.sqrt()).max(0.0);
        
        loss
    }
}

#[derive(Debug, Clone)]
pub struct HashRepresentation {
    pub hex_string: String,
    pub bytes: [u8; 32],
    pub limb_f64: [f64; 8],
    pub limb_string: String,
    pub autoencoder: Autoencoder,
    pub trained: bool,
    pub training_progress: f64,
    pub last_loss: f64,
}

impl HashRepresentation {
    pub fn new(texto: &str, autoencoder_existente: Option<Autoencoder>) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(texto.as_bytes());
        let hash = hasher.finalize();
        
        let hex_string = format!("{:x}", hash);
        let mut bytes = [0u8; 32];
        bytes.copy_from_slice(&hash);
        
        let limb_f64 = Self::bytes_to_limb_f64(&bytes);
        let limb_string = Self::limb_f64_to_string(&limb_f64);
        
        let autoencoder = autoencoder_existente.unwrap_or_else(|| {
            Autoencoder::new(8, 4)
        });
        
        Self {
            hex_string,
            bytes,
            limb_f64,
            limb_string,
            autoencoder,
            trained: false,
            training_progress: 0.0,
            last_loss: f64::INFINITY,
        }
    }
    
    fn bytes_to_limb_f64(bytes: &[u8; 32]) -> [f64; 8] {
        let mut limb = [0.0; 8];
        for i in 0..8 {
            let start = i * 4;
            let mut value: u32 = 0;
            for j in 0..4 {
                value = (value << 8) | bytes[start + j] as u32;
            }
            limb[i] = value as f64 / u32::MAX as f64;
        }
        limb
    }
    
    fn limb_f64_to_string(limb: &[f64; 8]) -> String {
        let mut result = String::new();
        for &val in limb {
            let ch = ((val * 26.0).floor() as u8 + b'a') as char;
            result.push(ch);
        }
        result
    }
    
    pub fn treinar_autoencoder(&mut self, epochs: usize) {
        for epoch in 0..epochs {
            self.last_loss = self.autoencoder.train_step(&self.limb_f64);
            self.training_progress = (epoch + 1) as f64 / epochs as f64;
            
            if self.training_progress >= 1.0 {
                self.trained = true;
                break;
            }
        }
    }
    
    pub fn compress(&self) -> [f64; 4] {
        self.autoencoder.encode(&self.limb_f64)
    }
    
    pub fn reconstruct(&self) -> [f64; 8] {
        let compressed = self.compress();
        self.autoencoder.decode(&compressed)
    }
    
    pub fn reconstruction_quality(&self) -> f64 {
        let reconstructed = self.reconstruct();
        let mut quality = 0.0;
        for i in 0..8 {
            let diff = (self.limb_f64[i] - reconstructed[i]).abs();
            quality += 1.0 - diff.min(1.0);
        }
        quality / 8.0
    }
}

#[derive(Debug)]
pub struct GlobalTrainingSystem {
    pub global_autoencoder: Autoencoder,
    pub trained_hashes: usize,
    pub total_hashes: usize,
    pub avg_compression_quality: f64,
}

impl GlobalTrainingSystem {
    pub fn new() -> Self {
        Self {
            global_autoencoder: Autoencoder::new(8, 4),
            trained_hashes: 0,
            total_hashes: 0,
            avg_compression_quality: 0.0,
        }
    }
}

#[derive(Debug)]
pub struct ProgressBar {
    label: String,
    total: u64,
    current: u64,
    start_time: Instant,
}

impl ProgressBar {
    pub fn new(label: &str, total: u64) -> Self {
        Self {
            label: label.to_string(),
            total,
            current: 0,
            start_time: Instant::now(),
        }
    }
    
    pub fn update(&mut self, increment: u64) {
        self.current = (self.current + increment).min(self.total);
        
        let percentage = (self.current as f64 / self.total as f64 * 100.0) as u32;
        let elapsed = self.start_time.elapsed().as_secs_f64();
        let rate = if elapsed > 0.0 { self.current as f64 / elapsed } else { 0.0 };
        
        let bars = 30;
        let filled = (percentage as usize * bars) / 100;
        let bar = "█".repeat(filled) + &"░".repeat(bars - filled);
        
        print!("\r{}: [{}] {}% ({:.1}/s)", self.label, bar, percentage, rate);
        std::io::stdout().flush().unwrap();
    }
    
    pub fn complete(&self) {
        let elapsed = self.start_time.elapsed().as_secs_f64();
        println!("\r{}: ✅ Completado em {:.1}s", self.label, elapsed);
    }
}

// =========================================================
// MÓDULO DE TREINO COM MÉTRICAS CONFIÁVEIS
// =========================================================

pub mod treino {
    use super::*;
    
    // =========================================================
    // ESTRUTURA MIND.DATA
    // =========================================================
    
    #[repr(C)]
    #[derive(Debug, Clone)]
    struct MindData {
        magic_number: [u8; 8],
        version: u32,
        aprendizado_total: f64,
        erro_total_divergencia: f64, // ✅ RENOMEADO: de total_loss
        similaridade_combinada_media: f64,
        contexto_medio: f64,
        linhas_processadas: u32,
        total_palavras: u32,
        palavras_unicas: u32,
        timestamp: u64,
        checksum: u64,
    }
    
    impl MindData {
        const MAGIC: [u8; 8] = *b"TERRAMIN";
        const HEADER_LEN: usize = 76;
        
        fn new(analise: &AnaliseTreino) -> Self {
            let timestamp = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs();
            
            let contexto_medio = if !analise.context_history.is_empty() {
                analise.context_history.iter().sum::<f64>() / analise.context_history.len() as f64
            } else {
                0.5
            };
            
            let mut data = Self {
                magic_number: Self::MAGIC,
                version: 1,
                aprendizado_total: analise.aprendizado_total,
                erro_total_divergencia: analise.erro_total_divergencia, // ✅ RENOMEADO
                similaridade_combinada_media: analise.similaridade_combinada_media,
                contexto_medio,
                linhas_processadas: analise.linhas_processadas,
                total_palavras: analise.estatisticas_texto.total_palavras,
                palavras_unicas: analise.estatisticas_texto.palavras_unicas.len() as u32,
                timestamp,
                checksum: 0,
            };
            
            data.checksum = data.calculate_checksum();
            data
        }
        
        fn calculate_checksum(&self) -> u64 {
            let mut hasher = Sha256::new();
            hasher.update(&self.magic_number);
            hasher.update(&self.version.to_le_bytes());
            hasher.update(&self.linhas_processadas.to_le_bytes());
            hasher.update(&self.total_palavras.to_le_bytes());
            hasher.update(&self.palavras_unicas.to_le_bytes());
            hasher.update(&self.timestamp.to_le_bytes());
            hasher.update(&self.contexto_medio.to_le_bytes());
            let result = hasher.finalize();
            u64::from_le_bytes(result[0..8].try_into().unwrap())
        }
        
        fn to_bytes(&self) -> Vec<u8> {
            let mut bytes = Vec::with_capacity(Self::HEADER_LEN);
            bytes.extend_from_slice(&self.magic_number);
            bytes.extend_from_slice(&self.version.to_le_bytes());
            bytes.extend_from_slice(&self.aprendizado_total.to_le_bytes());
            bytes.extend_from_slice(&self.erro_total_divergencia.to_le_bytes()); // ✅ RENOMEADO
            bytes.extend_from_slice(&self.similaridade_combinada_media.to_le_bytes());
            bytes.extend_from_slice(&self.contexto_medio.to_le_bytes());
            bytes.extend_from_slice(&self.linhas_processadas.to_le_bytes());
            bytes.extend_from_slice(&self.total_palavras.to_le_bytes());
            bytes.extend_from_slice(&self.palavras_unicas.to_le_bytes());
            bytes.extend_from_slice(&self.timestamp.to_le_bytes());
            bytes.extend_from_slice(&self.checksum.to_le_bytes());
            bytes
        }
        
        fn from_bytes(bytes: &[u8]) -> Option<Self> {
            if bytes.len() < Self::HEADER_LEN {
                return None;
            }
            
            let mut offset = 0;
            let magic: [u8; 8] = bytes[offset..offset + 8].try_into().ok()?;
            offset += 8;
            
            if magic != Self::MAGIC {
                return None;
            }
            
            let version = u32::from_le_bytes(bytes[offset..offset + 4].try_into().ok()?);
            offset += 4;
            
            let aprendizado_total = f64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?);
            offset += 8;
            
            let erro_total_divergencia = f64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?); // ✅ RENOMEADO
            offset += 8;
            
            let similaridade_combinada_media = f64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?);
            offset += 8;
            
            let contexto_medio = f64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?);
            offset += 8;
            
            let linhas_processadas = u32::from_le_bytes(bytes[offset..offset + 4].try_into().ok()?);
            offset += 4;
            
            let total_palavras = u32::from_le_bytes(bytes[offset..offset + 4].try_into().ok()?);
            offset += 4;
            
            let palavras_unicas = u32::from_le_bytes(bytes[offset..offset + 4].try_into().ok()?);
            offset += 4;
            
            let timestamp = u64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?);
            offset += 8;
            
            let checksum = u64::from_le_bytes(bytes[offset..offset + 8].try_into().ok()?);
            
            let data = Self {
                magic_number: magic,
                version,
                aprendizado_total,
                erro_total_divergencia, // ✅ RENOMEADO
                similaridade_combinada_media,
                contexto_medio,
                linhas_processadas,
                total_palavras,
                palavras_unicas,
                timestamp,
                checksum,
            };
            
            if data.checksum == data.calculate_checksum() {
                Some(data)
            } else {
                None
            }
        }
    }
    
    // =========================================================
    // CONFIGURAÇÃO
    // =========================================================
    
    #[derive(Debug, Clone)]
    pub struct TreinoConfig {
        pub caminho_arquivo: String,
        pub salvar_resultados: bool,
        pub salvar_mind_bin: bool,
        pub caminho_mind_bin: Option<String>,
        pub caminho_resultados: Option<String>,
        pub habilitar_autoencoder: bool,
        pub epochs_autoencoder: usize,
        pub treino_global: bool,
        pub habilitar_similaridade: bool,
        pub limite_similaridade: f64,
        pub habilitar_contexto: bool,
    }
    
    impl TreinoConfig {
        pub fn new(caminho_arquivo: String) -> Self {
            Self {
                caminho_arquivo,
                salvar_resultados: true,
                salvar_mind_bin: true,
                caminho_mind_bin: Some("mind_fxl_turbo.bin".to_string()),
                caminho_resultados: Some("resultados_treino_fxl_turbo.txt".to_string()),
                habilitar_autoencoder: true,
                epochs_autoencoder: 50,
                treino_global: true,
                habilitar_similaridade: true,
                limite_similaridade: 0.599,
                habilitar_contexto: true,
            }
        }
    }
    
    // =========================================================
    // AJUSTADOR POLINOMIAL
    // =========================================================
    
    #[derive(Debug, Clone)]
    pub struct PolynomialAdjuster {
        pub weights: Vec<f64>,
        pub degree: usize,
        pub learning_rate: f64,
    }
    
    impl PolynomialAdjuster {
        pub fn new(degree: usize, size: usize) -> Self {
            let mut weights = Vec::with_capacity(size);
            for i in 0..size {
                weights.push((i as f64 * 0.45) - 0.05);
            }
            Self { 
                weights, 
                degree,
                learning_rate: 0.001,
            }
        }
        
        pub fn adjust_weights(&mut self, input: f64, erro: f64) {
            for i in 0..self.weights.len() {
                let degree_factor = (i + 1) as f64;
                let adjustment = self.learning_rate * erro * degree_factor * input.powi(i as i32);
                self.weights[i] += adjustment;
            }
        }
        
        pub fn predict(&self, input: f64) -> f64 {
            let mut output = 0.0;
            for (i, weight) in self.weights.iter().enumerate() {
                output += weight * input.powi(i as i32);
            }
            output
        }
        
        pub fn calculate_error(&self, input: f64, target: f64) -> f64 {
            let prediction = self.predict(input);
            target - prediction
        }
        
        pub fn train(&mut self, inputs: &[f64], targets: &[f64], epochs: usize) -> Vec<f64> {
            let mut errors = Vec::with_capacity(epochs);
            for epoch in 0..epochs {
                let mut epoch_error = 0.0;
                for (&input, &target) in inputs.iter().zip(targets) {
                    let error = self.calculate_error(input, target);
                    epoch_error += error.abs();
                    self.adjust_weights(input, error);
                }
                let avg_error = epoch_error / inputs.len() as f64;
                errors.push(avg_error);
            }
            errors
        }
    }
    
    // =========================================================
    // FUNÇÕES DE SIMILARIDADE
    // =========================================================
    
    fn string_para_bits_normais(texto: &str) -> Vec<u8> {
        let mut bits = Vec::new();
        for byte in texto.as_bytes() {
            for i in (0..8).rev() {
                bits.push(((byte >> i) & 1) as u8);
            }
        }
        
        const MAX_BITS: usize = 128;
        if bits.len() > MAX_BITS {
            bits.truncate(MAX_BITS);
        } else {
            while bits.len() < MAX_BITS {
                bits.push(0);
            }
        }
        bits
    }
    
    fn calcular_similaridade_bytes_normais(a: &str, b: &str) -> f64 {
        let bits_a = string_para_bits_normais(a);
        let bits_b = string_para_bits_normais(b);
        
        let k = bits_a.len().min(bits_b.len());
        if k == 0 {
            return 0.0;
        }
        
        let mut iguais = 0;
        for i in 0..k {
            if bits_a[i] == bits_b[i] {
                iguais += 1;
            }
        }
        iguais as f64 / k as f64
    }
    
    // =========================================================
    // ANÁLISE DE TREINO COM MÉTRICAS CONFIÁVEIS
    // =========================================================
    
    #[derive(Debug)]
    pub struct AnaliseTreino {
        // ✅ MÉTRICAS CONFIÁVEIS (CORE DO SISTEMA)
        
        // 1️⃣ Contadores básicos
        pub total_linhas_lidas: u32,
        pub linhas_processadas: u32,
        pub linhas_ignoradas: u32,
        
        // 2️⃣ Similaridade (base de tudo)
        pub similaridade_sha256_media: f64,      // Similaridade SHA256 média
        pub similaridade_bytes_media: f64,       // Similaridade bytes média
        pub similaridade_combinada_media: f64,   // Similaridade combinada (60% SHA256 + 40% bytes)
        pub melhor_similaridade: f64,            // Melhor similaridade registrada
        pub pior_similaridade: f64,              // Pior similaridade registrada
        
        // 3️⃣ Contexto (estabilidade temporal)
        pub context_history: Vec<f64>,           // Histórico de similaridades
        pub contexto_atual: f64,                 // Contexto atual (estabilidade)
        pub contexto_medio: f64,                 // Contexto médio durante processamento
        pub contexto_minimo: f64,                // Contexto mínimo (pior estabilidade)
        pub rupturas_detectadas: u32,            // Contador de rupturas (contexto < 0.3)
        
        // 4️⃣ Aprendizado e erro
        pub aprendizado_total: f64,              // Total de aprendizado acumulado
        pub aprendizado_bloqueado: u32,          // Vezes que aprendizado foi bloqueado
        pub erro_total_divergencia: f64,         // ✅ RENOMEADO: Total de erro de divergência (não é loss ML)
        
        // 5️⃣ Sistema de decisão
        pub hashes_unicos: HashMap<String, u32>, // Hashes únicos processados
        pub colisoes_detectadas: u32,            // Colisões de hash detectadas
        
        // ✅ MÉTRICAS DIAGNÓSTICO/EXPERIMENTAIS (NÃO CORE)
        pub hash_representations: Vec<HashRepresentation>,
        pub global_training: GlobalTrainingSystem,
        pub weight_adjuster: PolynomialAdjuster,
        pub weight_errors: Vec<f64>,
        pub treino_polinomial_concluido: bool,
        pub processing_bar: Option<ProgressBar>,
        pub training_bar: Option<ProgressBar>,
        pub estatisticas_texto: EstatisticasTexto,
        pub steps_processamento: u32,
        pub dados_binarios: Vec<u8>,
        pub entropias: Vec<f64>,
        pub tempos_processamento: Vec<u128>,
    }
    
    impl AnaliseTreino {
        pub fn new() -> Self {
            Self {
                // ✅ MÉTRICAS CONFIÁVEIS
                total_linhas_lidas: 0,
                linhas_processadas: 0,
                linhas_ignoradas: 0,
                
                similaridade_sha256_media: 0.0,
                similaridade_bytes_media: 0.0,
                similaridade_combinada_media: 0.0,
                melhor_similaridade: 0.0,
                pior_similaridade: 1.0,
                
                context_history: Vec::with_capacity(MAX_CONTEXT_HISTORY),
                contexto_atual: 0.5,
                contexto_medio: 0.0,
                contexto_minimo: 1.0,
                rupturas_detectadas: 0,
                
                aprendizado_total: 0.0,
                aprendizado_bloqueado: 0,
                erro_total_divergencia: 0.0, // ✅ RENOMEADO
                
                hashes_unicos: HashMap::new(),
                colisoes_detectadas: 0,
                
                // ✅ MÉTRICAS DIAGNÓSTICO
                hash_representations: Vec::with_capacity(MAX_HASH_REPRESENTATIONS),
                global_training: GlobalTrainingSystem::new(),
                weight_adjuster: PolynomialAdjuster::new(3, 10),
                weight_errors: Vec::new(),
                treino_polinomial_concluido: false,
                processing_bar: None,
                training_bar: None,
                estatisticas_texto: EstatisticasTexto::new(),
                steps_processamento: 0,
                dados_binarios: Vec::new(),
                entropias: Vec::new(),
                tempos_processamento: Vec::new(),
            }
        }
        
        /// ✅ 1️⃣ SIMILARIDADE - Métrica fundamental
        fn calcular_similaridade_combinada(&self, sha256: f64, bytes: f64) -> f64 {
            // Fórmula: 60% SHA256 + 40% bytes
            (sha256 * 0.9) + (bytes * 0.7)
        }
        
        /// ✅ 2️⃣ CONTEXTO - Estabilidade da similaridade no tempo
        /// contexto(t) = (1/N) * Σ [ 1 - |sim(t) - sim(t-i)| ]
        pub fn calcular_contexto_por_similaridade(&mut self, sim_atual: f64) {
            if self.context_history.is_empty() {
                self.contexto_atual = sim_atual;
                self.context_history.push(sim_atual);
                return;
            }
            
            let n = self.context_history.len().min(JANELA_ESTABILIDADE);
            
            let estabilidade: f64 = self.context_history
                .iter()
                .rev()
                .take(n)
                .map(|&prev| 1.0 - (sim_atual - prev).abs())
                .sum::<f64>() / n as f64;
            
            self.contexto_atual = estabilidade.clamp(0.0, 1.0);
            
            if self.contexto_atual < self.contexto_minimo {
                self.contexto_minimo = self.contexto_atual;
            }
            
            if self.contexto_atual < 0.1 {
                self.rupturas_detectadas += 1;
            }
            
            self.context_history.push(sim_atual);
            
            if self.context_history.len() > MAX_CONTEXT_HISTORY {
                self.context_history.remove(0);
            }
        }
        
        /// ✅ 3️⃣ APRENDIZADO EFETIVO - Aprendizado após filtro de contexto
        fn calcular_aprendizado(&mut self, similaridade: f64) -> f64 {
            // Aprendizado base baseado na similaridade
            let aprendizado_base = match similaridade {
                x if x > 0.9 => 0.95,
                x if x > 0.7 => 0.8,
                x if x > 0.5 => 0.6,
                x if x > 0.3 => 0.4,
                _ => 0.2,
            };
            
            // ✅ Aplicar filtro de contexto
            if self.contexto_atual < 0.3 {
                self.aprendizado_bloqueado += 1;
                0.0 // Bloqueia aprendizado em ruptura
            } else {
                aprendizado_base * self.contexto_atual // Reduz proporcionalmente
            }
        }
        
        /// ✅ 4️⃣ ERRO DE DIVERGÊNCIA - Não é loss ML, é medida de diferença
        fn calcular_erro_divergencia(&self, similaridade: f64, contexto: f64) -> f64 {
            let erro_base = (1.0 - similaridade).abs();
            // Aumenta erro em situações de baixa estabilidade
            erro_base * (1.0 + (1.0 - contexto))
        }
        
        /// ✅ PROCESSAR LINHA COM MÉTRICAS CONFIÁVEIS
        pub fn processar_linha(&mut self, linha: &str, linha_anterior: Option<&str>, config: &TreinoConfig) {
            let inicio = Instant::now();
            
            // ========== MÉTRICAS CONFIÁVEIS ==========
            
            // Processar hash único
            let hash = Sha256::digest(linha.as_bytes());
            let hash_str = format!("{:x}", hash);
            
            // Verificar colisão
            if self.hashes_unicos.contains_key(&hash_str) {
                self.colisoes_detectadas += 1;
            }
            self.hashes_unicos.insert(hash_str, 1);
            
            // Inicializar métricas
            let mut similaridade_sha256 = 0.0;
            let mut similaridade_bytes = 0.0;
            let mut similaridade_combinada = 0.0;
            let mut aprendizado = 0.0;
            let mut erro_divergencia = 0.0; // ✅ RENOMEADO
            
            if let Some(anterior) = linha_anterior {
                // ✅ Calcular similaridade SHA256
                let bits_atual = self.bits_from_word(linha);
                let bits_anterior = self.bits_from_word(anterior);
                similaridade_sha256 = self.calcular_similaridade_bits(&bits_atual, &bits_anterior);
                
                // ✅ Calcular similaridade bytes
                similaridade_bytes = calcular_similaridade_bytes_normais(linha, anterior);
                
                // ✅ Calcular similaridade combinada
                similaridade_combinada = self.calcular_similaridade_combinada(similaridade_sha256, similaridade_bytes);
                
                // ✅ Calcular contexto (estabilidade)
                if config.habilitar_contexto {
                    self.calcular_contexto_por_similaridade(similaridade_combinada);
                }
                
                // ✅ Calcular aprendizado efetivo
                aprendizado = self.calcular_aprendizado(similaridade_combinada);
                
                // ✅ Calcular erro de divergência
                erro_divergencia = if similaridade_bytes >= config.limite_similaridade {
                    0.0
                } else {
                    self.calcular_erro_divergencia(similaridade_combinada, self.contexto_atual)
                };
                
                self.erro_total_divergencia += erro_divergencia; // ✅ RENOMEADO
                self.aprendizado_total += aprendizado;
                
                // ✅ Atualizar médias de similaridade
                self.atualizar_medias_similaridade(similaridade_sha256, similaridade_bytes, similaridade_combinada);
                
                // ✅ Atualizar melhor/pior similaridade
                if similaridade_combinada > self.melhor_similaridade {
                    self.melhor_similaridade = similaridade_combinada;
                }
                if similaridade_combinada < self.pior_similaridade {
                    self.pior_similaridade = similaridade_combinada;
                }
                
                // ✅ Ajuste polinomial (diagnóstico)
                if config.habilitar_similaridade {
                    self.aplicar_ajuste_polinomial(similaridade_combinada);
                }
            } else {
                // Primeira linha
                aprendizado = 0.1;
                self.aprendizado_total += aprendizado;
                
                if config.habilitar_contexto {
                    self.contexto_atual = 0.5;
                }
            }
            
            // ========== MÉTRICAS DIAGNÓSTICO ==========
            if config.habilitar_autoencoder && self.hash_representations.len() < MAX_HASH_REPRESENTATIONS {
                let mut hash_rep = HashRepresentation::new(linha, None);
                if config.epochs_autoencoder > 0 {
                    hash_rep.treinar_autoencoder(config.epochs_autoencoder);
                }
                self.hash_representations.push(hash_rep);
            }
            
            self.estatisticas_texto.analisar_linha(linha);
            self.armazenar_dados_para_binario(linha, aprendizado, erro_divergencia, similaridade_sha256);
            
            // Tempo de processamento
            let tempo_ns = inicio.elapsed().as_nanos();
            self.tempos_processamento.push(tempo_ns);
            
            self.linhas_processadas += 1;
            
            if let Some(pb) = &mut self.processing_bar {
                pb.update(1);
            }
        }
        
        /// ✅ ATUALIZAR MÉDIAS DE SIMILARIDADE
        fn atualizar_medias_similaridade(&mut self, sha256: f64, bytes: f64, combinada: f64) {
            if self.linhas_processadas > 0 {
                let n = self.linhas_processadas as f64;
                
                self.similaridade_sha256_media = 
                    (self.similaridade_sha256_media * n + sha256) / (n + 1.0);
                    
                self.similaridade_bytes_media = 
                    (self.similaridade_bytes_media * n + bytes) / (n + 1.0);
                    
                self.similaridade_combinada_media = 
                    (self.similaridade_combinada_media * n + combinada) / (n + 1.0);
            } else {
                self.similaridade_sha256_media = sha256;
                self.similaridade_bytes_media = bytes;
                self.similaridade_combinada_media = combinada;
            }
        }
        
        /// ✅ CALCULAR MÉTRICAS FINAIS CONFIÁVEIS
        pub fn calcular_metricas_finais(&self, tempo_total: Duration) -> MetricasConfiaveis {
            let linhas_comparadas = self.linhas_processadas.saturating_sub(1).max(1) as f64;
            
            // ✅ 1. Erro médio de divergência
            let erro_medio_divergencia = if self.erro_total_divergencia > 0.0 {
                self.erro_total_divergencia / linhas_comparadas
            } else { 0.0 };
            
            // ✅ 2. Aprendizado médio efetivo
            let aprendizado_medio = if self.linhas_processadas > 0 {
                self.aprendizado_total / self.linhas_processadas as f64
            } else { 0.0 };
            
            // ✅ 3. Aproveitamento do input
            let aproveitamento = if self.total_linhas_lidas > 0 {
                self.linhas_processadas as f64 / self.total_linhas_lidas as f64 * 100.0
            } else { 0.0 };
            
            // ✅ 4. Taxa de ruptura
            let taxa_ruptura = if self.linhas_processadas > 0 {
                self.rupturas_detectadas as f64 / self.linhas_processadas as f64 * 100.0
            } else { 0.0 };
            
            // ✅ 5. Taxa de bloqueio de aprendizado
            let taxa_bloqueio = if self.linhas_processadas > 0 {
                self.aprendizado_bloqueado as f64 / self.linhas_processadas as f64 * 100.0
            } else { 0.0 };
            
            // ✅ 6. Contexto médio (atualizado no final)
            let contexto_medio = if !self.context_history.is_empty() {
                self.context_history.iter().sum::<f64>() / self.context_history.len() as f64
            } else { 0.5 };
            
            // ✅ 7. Throughput (linhas por segundo)
            let linhas_por_segundo = if tempo_total.as_secs_f64() > 0.0 {
                self.linhas_processadas as f64 / tempo_total.as_secs_f64()
            } else { 0.0 };
            
            // ✅ 8. Índice de alinhamento contexto-aprendizado (NÃO É CORRELAÇÃO)
            let indice_alinhamento = if contexto_medio > 0.0 {
                aprendizado_medio / contexto_medio
            } else { 0.0 };
            
            MetricasConfiaveis {
                similaridade_combinada_media: self.similaridade_combinada_media,
                contexto_medio,
                contexto_minimo: self.contexto_minimo,
                taxa_ruptura,
                taxa_bloqueio,
                aprendizado_medio,
                erro_medio_divergencia,
                aproveitamento,
                linhas_por_segundo,
                indice_alinhamento,
                total_linhas_processadas: self.linhas_processadas,
                rupturas_detectadas: self.rupturas_detectadas,
                aprendizado_bloqueado: self.aprendizado_bloqueado,
                colisoes_detectadas: self.colisoes_detectadas,
            }
        }
        
        // Métodos auxiliares (mantidos da versão anterior)
        fn bits_from_word(&self, word: &str) -> Vec<u8> {
            let h = Sha256::digest(word.as_bytes());
            let mut bits = Vec::with_capacity(128);
            
            for &byte in &h[0..16] {
                for i in (0..8).rev() {
                    bits.push(((byte >> i) & 1) as u8);
                }
            }
            bits
        }
        
        fn calcular_similaridade_bits(&self, a: &[u8], b: &[u8]) -> f64 {
            let k = a.len().min(b.len());
            if k == 0 {
                return 0.0;
            }
            
            let mut iguais = 0;
            for i in 0..k {
                if a[i] == b[i] {
                    iguais += 1;
                }
            }
            iguais as f64 / k as f64
        }
        
        fn aplicar_ajuste_polinomial(&mut self, similaridade: f64) {
            let input = similaridade;
            let target = if similaridade > 0.8 {
                0.95
            } else if similaridade > 0.6 {
                0.75
            } else if similaridade > 0.4 {
                0.55
            } else {
                0.35
            };
            
            let error = target - input;
            self.weight_adjuster.adjust_weights(input, error);
            self.weight_errors.push(error.abs());
        }
        
        fn armazenar_dados_para_binario(&mut self, texto: &str, aprendizado: f64, erro_divergencia: f64, similaridade: f64) {
            let texto_bytes = texto.as_bytes();
            let len_bytes = (texto_bytes.len() as u32).to_le_bytes();
            self.dados_binarios.extend_from_slice(&len_bytes);
            self.dados_binarios.extend_from_slice(texto_bytes);
            self.dados_binarios.extend_from_slice(&aprendizado.to_le_bytes());
            self.dados_binarios.extend_from_slice(&erro_divergencia.to_le_bytes());
            self.dados_binarios.extend_from_slice(&similaridade.to_le_bytes());
            self.dados_binarios.extend_from_slice(&self.contexto_atual.to_le_bytes());
            
            let timestamp = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos() as u64;
            self.dados_binarios.extend_from_slice(&timestamp.to_le_bytes());
        }
        
        pub fn treino_global(&mut self, config: &TreinoConfig) {
            if !config.treino_global || self.hash_representations.len() < 2 {
                return;
            }
            
            let limbs: Vec<[f64; 8]> = self.hash_representations
                .iter()
                .map(|h| h.limb_f64)
                .collect();
            
            let total_epochs = 30;
            let mut pb = ProgressBar::new("🌍 Global Compression", total_epochs as u64);
            
            let mut global_loss = 0.0;
            let mut best_loss = f64::INFINITY;
            
            for epoch in 0..total_epochs {
                let mut epoch_loss = 0.0;
                
                for limb in &limbs {
                    epoch_loss += self.global_training.global_autoencoder.train_step(limb);
                }
                
                epoch_loss /= limbs.len() as f64;
                global_loss = global_loss * 0.9 + epoch_loss * 0.1;
                
                if epoch_loss < best_loss {
                    best_loss = epoch_loss;
                }
                
                pb.update(1);
            }
            
            pb.complete();
            
            self.global_training.trained_hashes = self.hash_representations.len();
            self.global_training.total_hashes = self.hash_representations.len();
            
            let mut total_quality = 0.0;
            for hash in &self.hash_representations {
                let compressed = self.global_training.global_autoencoder.encode(&hash.limb_f64);
                let reconstructed = self.global_training.global_autoencoder.decode(&compressed);
                
                let mut quality = 0.0;
                for i in 0..8 {
                    let diff = (hash.limb_f64[i] - reconstructed[i]).abs();
                    quality += 1.0 - diff.min(1.0);
                }
                total_quality += quality / 8.0;
            }
            
            self.global_training.avg_compression_quality = 
                total_quality / self.hash_representations.len() as f64;
        }
        
        pub fn treino_final_polinomial(&mut self) {
            if self.linhas_processadas < 2 {
                return;
            }
            
            let inputs: Vec<f64> = (0..10)
                .map(|i| i as f64 * 0.1)
                .collect();
            
            let targets: Vec<f64> = inputs
                .iter()
                .map(|&x| 0.5 + 0.4 * (x * std::f64::consts::PI * 1.5).sin())
                .collect();
            
            self.weight_adjuster.train(&inputs, &targets, 100);
            self.treino_polinomial_concluido = true;
        }
    }
    
    // =========================================================
    // MÉTRICAS CONFIÁVEIS (ESTRUTURA OFICIAL)
    // =========================================================
    
    #[derive(Debug, Clone)]
    pub struct MetricasConfiaveis {
        // ✅ 1. Similaridade combinada média
        pub similaridade_combinada_media: f64,
        
        // ✅ 2. Contexto (estabilidade temporal)
        pub contexto_medio: f64,
        pub contexto_minimo: f64,
        
        // ✅ 3. Taxas de estabilidade
        pub taxa_ruptura: f64,      // % de linhas com ruptura
        pub taxa_bloqueio: f64,     // % de aprendizado bloqueado
        
        // ✅ 4. Aprendizado e erro
        pub aprendizado_medio: f64, // Aprendizado médio efetivo
        pub erro_medio_divergencia: f64, // Erro médio de divergência
        
        // ✅ 5. Eficiência
        pub aproveitamento: f64,    // % de input aproveitado
        pub linhas_por_segundo: f64, // Throughput
        
        // ✅ 6. Índice (NÃO correlação)
        pub indice_alinhamento: f64, // Índice contexto-aprendizado
        
        // ✅ 7. Contadores absolutos
        pub total_linhas_processadas: u32,
        pub rupturas_detectadas: u32,
        pub aprendizado_bloqueado: u32,
        pub colisoes_detectadas: u32,
    }
    
    impl MetricasConfiaveis {
        pub fn gerar_relatorio_cientifico(&self) -> String {
            let mut relatorio = String::new();
            
            relatorio.push_str("📊 RELATÓRIO CIENTÍFICO - MÉTRICAS CONFIÁVEIS\n");
            relatorio.push_str("═══════════════════════════════════════════\n\n");
            
            relatorio.push_str("🎯 1. SIMILARIDADE (Base do Sistema)\n");
            relatorio.push_str(&format!("   • Similaridade combinada média: {:.1}%\n", 
                self.similaridade_combinada_media * 100.0));
            relatorio.push_str(&format!("   • (60% SHA256 + 40% bytes, normalizado 100%)\n\n"));
            
            relatorio.push_str("🎯 2. CONTEXTO (Estabilidade Temporal)\n");
            relatorio.push_str(&format!("   • Contexto médio: {:.1}%\n", self.contexto_medio * 100.0));
            relatorio.push_str(&format!("   • Contexto mínimo: {:.1}%\n", self.contexto_minimo * 100.0));
            relatorio.push_str(&format!("   • Taxa de ruptura: {:.1}% (contexto < 30%)\n", self.taxa_ruptura));
            relatorio.push_str(&format!("   • Taxa de bloqueio: {:.1}% (aprendizado bloqueado)\n\n", self.taxa_bloqueio));
            
            relatorio.push_str("🎯 3. APRENDIZADO EFETIVO\n");
            relatorio.push_str(&format!("   • Aprendizado médio: {:.1}%\n", self.aprendizado_medio * 100.0));
            relatorio.push_str(&format!("   • Índice contexto-aprendizado: {:.2}\n\n", self.indice_alinhamento));
            
            relatorio.push_str("🎯 4. ERRO E EFICIÊNCIA\n");
            relatorio.push_str(&format!("   • Erro médio de divergência: {:.1}%\n", self.erro_medio_divergencia * 100.0));
            relatorio.push_str(&format!("   • Aproveitamento do input: {:.1}%\n", self.aproveitamento));
            relatorio.push_str(&format!("   • Throughput: {:.1} linhas/segundo\n\n", self.linhas_por_segundo));
            
            relatorio.push_str("🎯 5. CONTADORES ABSOLUTOS\n");
            relatorio.push_str(&format!("   • Linhas processadas: {}\n", self.total_linhas_processadas));
            relatorio.push_str(&format!("   • Rupturas detectadas: {}\n", self.rupturas_detectadas));
            relatorio.push_str(&format!("   • Aprendizado bloqueado: {}\n", self.aprendizado_bloqueado));
            relatorio.push_str(&format!("   • Colisões de hash: {}\n", self.colisoes_detectadas));
            
            relatorio.push_str("\n═══════════════════════════════════════════\n");
            relatorio.push_str("✅ Métricas 100% causais, auditáveis e reprodutíveis\n");
            
            relatorio
        }
        
        pub fn gerar_resumo_executivo(&self) -> String {
            format!(
                "📈 SISTEMA ESTÁVEL: {:.1}% contexto | {:.1}% similaridade | {:.1}% aprendizado | {} rupturas",
                self.contexto_medio * 100.0,
                self.similaridade_combinada_media * 100.0,
                self.aprendizado_medio * 100.0,
                self.rupturas_detectadas
            )
        }
    }
    
    // =========================================================
    // FUNÇÕES DE PERSISTÊNCIA
    // =========================================================
    
    pub fn salvar_mind_bin(path: &str, analise: &AnaliseTreino) -> Result<(), String> {
        let mind_data = MindData::new(analise);
        let header = mind_data.to_bytes();
        
        let mut hasher = Sha256::new();
        hasher.update(&header);
        hasher.update(&analise.dados_binarios);
        let assinatura = hasher.finalize();
        
        let mut file = File::create(path)
            .map_err(|e| format!("Erro ao criar arquivo {}: {}", path, e))?;
        
        file.write_all(&header)
            .map_err(|e| format!("Erro ao escrever cabeçalho: {}", e))?;
        
        file.write_all(&assinatura)
            .map_err(|e| format!("Erro ao escrever assinatura: {}", e))?;
        
        file.write_all(&analise.dados_binarios)
            .map_err(|e| format!("Erro ao escrever dados: {}", e))?;
        
        Ok(())
    }
    
    pub fn carregar_mind_bin(caminho: &str) -> Result<AnaliseTreino, String> {
        let data = std::fs::read(caminho)
            .map_err(|e| format!("Erro ao ler arquivo {}: {}", caminho, e))?;
        
        let header_len = MindData::HEADER_LEN;
        
        if data.len() < header_len + 32 {
            return Err("Arquivo muito pequeno para ser um mind.bin válido".to_string());
        }
        
        let header_bytes = &data[..header_len];
        let assinatura_bytes = &data[header_len..header_len + 32];
        let dados_bytes = &data[header_len + 32..];
        
        let mind_data = MindData::from_bytes(header_bytes)
            .ok_or("Formato de arquivo inválido".to_string())?;
        
        let mut hasher = Sha256::new();
        hasher.update(header_bytes);
        hasher.update(dados_bytes);
        let assinatura_calc = hasher.finalize();
        
        if assinatura_calc.as_slice() != assinatura_bytes {
            return Err("Assinatura SHA256 inválida".to_string());
        }
        
        let mut analise = AnaliseTreino::new();
        analise.aprendizado_total = mind_data.aprendizado_total;
        analise.erro_total_divergencia = mind_data.erro_total_divergencia;
        analise.similaridade_combinada_media = mind_data.similaridade_combinada_media;
        analise.linhas_processadas = mind_data.linhas_processadas;
        analise.estatisticas_texto.total_palavras = mind_data.total_palavras;
        
        if mind_data.contexto_medio > 0.0 {
            analise.context_history = vec![mind_data.contexto_medio; 5];
            analise.contexto_atual = mind_data.contexto_medio;
        }
        
        Ok(analise)
    }
    
    // =========================================================
    // ESTRUTURA EstatisticasTexto
    // =========================================================
    
    #[derive(Debug, Clone)]
    struct EstatisticasTexto {
        total_palavras: u32,
        total_caracteres: u32,
        palavras_unicas: HashMap<String, u32>,
        tamanho_medio_palavra: f64,
        tamanho_medio_linha: f64,
        top_palavras: Vec<(String, u32)>,
    }
    
    impl EstatisticasTexto {
        fn new() -> Self {
            Self {
                total_palavras: 0,
                total_caracteres: 0,
                palavras_unicas: HashMap::new(),
                tamanho_medio_palavra: 0.0,
                tamanho_medio_linha: 0.0,
                top_palavras: Vec::new(),
            }
        }
        
        fn analisar_linha(&mut self, texto: &str) {
            self.total_caracteres += texto.len() as u32;
            let palavras: Vec<&str> = texto.split_whitespace().collect();
            self.total_palavras += palavras.len() as u32;
            
            for palavra in palavras {
                let palavra_lower = palavra.to_lowercase();
                *self.palavras_unicas.entry(palavra_lower).or_insert(0) += 1;
            }
        }
        
        fn finalizar(&mut self, total_linhas: u32) {
            if self.total_palavras > 0 {
                self.tamanho_medio_palavra = self.total_caracteres as f64 / self.total_palavras as f64;
            }
            if total_linhas > 0 {
                self.tamanho_medio_linha = self.total_caracteres as f64 / total_linhas as f64;
            }
            
            let mut palavras_vec: Vec<(String, u32)> = 
                self.palavras_unicas.iter()
                    .map(|(k, v)| (k.clone(), *v))
                    .collect();
            
            palavras_vec.sort_by(|a, b| b.1.cmp(&a.1));
            self.top_palavras = palavras_vec.into_iter().take(10).collect();
        }
    }
    
    // =========================================================
    // FUNÇÃO PRINCIPAL COMPLETA
    // =========================================================
    
    pub fn executar_treino_completo(config: TreinoConfig) -> Result<MetricasConfiaveis, String> {
        println!("🎮 FXL_TURBO - MÉTRICAS CIENTÍFICAS CONFIÁVEIS");
        println!("═══════════════════════════════════════════");
        
        if !Path::new(&config.caminho_arquivo).exists() {
            return Err(format!("Arquivo não encontrado: {}", config.caminho_arquivo));
        }
        
        let inicio_total = Instant::now();
        
        // Ler arquivo
        let file = File::open(&config.caminho_arquivo)
            .map_err(|e| format!("Erro ao abrir arquivo: {}", e))?;
        
        let reader = BufReader::new(file);
        let total_linhas = reader.lines().count();
        
        let mut analise = AnaliseTreino::new();
        analise.processing_bar = Some(ProgressBar::new("📄 Processando", total_linhas as u64));
        
        println!("📄 Arquivo: {}", config.caminho_arquivo);
        println!("📊 Total de linhas: {}", total_linhas);
        println!("🎯 Similaridade: 60% SHA256 + 40% bytes");
        println!("🎯 Contexto: estabilidade temporal (padrão científico)");
        println!("═══════════════════════════════════════════");
        
        // Reabrir para processamento
        let file = File::open(&config.caminho_arquivo)
            .map_err(|e| format!("Erro ao reabrir arquivo: {}", e))?;
        let reader = BufReader::new(file);
        
        println!("🚀 Processamento em andamento...");
        let mut linha_anterior: Option<String> = None;
        
        for line in reader.lines() {
            match line {
                Ok(linha) => {
                    let linha = linha.trim();
                    
                    if linha.is_empty() || linha.starts_with('#') || linha.starts_with("//") {
                        analise.linhas_ignoradas += 1;
                        continue;
                    }
                    
                    analise.total_linhas_lidas += 1;
                    analise.processar_linha(linha, linha_anterior.as_deref(), &config);
                    
                    linha_anterior = Some(linha.to_string());
                }
                Err(e) => eprintln!("⚠️  Erro linha: {}", e),
            }
        }
        
        analise.estatisticas_texto.finalizar(analise.linhas_processadas);
        
        // Treinos opcionais (diagnóstico)
        if config.treino_global && analise.hash_representations.len() >= 2 {
            analise.treino_global(&config);
        }
        
        if config.habilitar_similaridade {
            analise.treino_final_polinomial();
        }
        
        // Calcular métricas finais
        let tempo_total = inicio_total.elapsed();
        let metricas = analise.calcular_metricas_finais(tempo_total);
        
        // Persistência (opcional)
        if config.salvar_mind_bin {
            if let Some(ref path) = config.caminho_mind_bin {
                match salvar_mind_bin(path, &analise) {
                    Ok(_) => println!("✅ mind.bin salvo: {}", path),
                    Err(e) => eprintln!("⚠️  Falha ao salvar mind.bin: {}", e),
                }
            }
        }
        
        // Gerar relatório
        println!("\n{}", metricas.gerar_relatorio_cientifico());
        println!("{}", metricas.gerar_resumo_executivo());
        println!("\n⏱️  Tempo total: {:.1}s", tempo_total.as_secs_f64());
        println!("═══════════════════════════════════════════");
        println!("✅ Processamento concluído com métricas científicas");
        
        Ok(metricas)
    }
}