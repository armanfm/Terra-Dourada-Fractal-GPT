use std::{env, fs, path::Path, process};

use terra_dourada_gpt::fxl_turbo::treino::{executar_treino_completo, TreinoConfig};

fn ensure_parent_dir(path: &str) -> Result<(), String> {
    let p = Path::new(path);
    if let Some(parent) = p.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("falha create_dir_all({:?}): {}", parent, e))?;
        }
    }
    Ok(())
}

fn file_size(path: &str) -> Option<u64> {
    fs::metadata(path).ok().map(|m| m.len())
}

fn main() {
    let args: Vec<String> = env::args().collect();

    if args.len() < 2 {
        eprintln!("uso: treino <caminho_txt>");
        process::exit(2);
    }

    let caminho_txt = args[1].trim().to_string();
    if caminho_txt.is_empty() {
        eprintln!("caminho_txt vazio");
        process::exit(2);
    }

    // destino soberano por request (Go seta)
    let mind_path = env::var("TD_MIND_PATH").unwrap_or_else(|_| "src/data/mind.bin".to_string());
    let result_path =
        env::var("TD_RESULT_PATH").unwrap_or_else(|_| "src/data/resultados_fxl.txt".to_string());

    // garante diretórios do job (temp)
    if let Err(e) = ensure_parent_dir(&mind_path) {
        eprintln!("❌ {}", e);
        process::exit(1);
    }
    if let Err(e) = ensure_parent_dir(&result_path) {
        eprintln!("❌ {}", e);
        process::exit(1);
    }

    println!("🧩 TREINO BIN (env-aware)");
    println!("🧠 TD_MIND_PATH = {}", mind_path);
    println!("📄 TD_RESULT_PATH = {}", result_path);

    let mut config = TreinoConfig::new(caminho_txt);

    // tenta passar pro core (se ele respeitar, ótimo)
    config.salvar_mind_bin = true;
    config.caminho_mind_bin = Some(mind_path.clone());

    config.salvar_resultados = true;
    config.caminho_resultados = Some(result_path.clone());

    config.habilitar_autoencoder = true;
    config.epochs_autoencoder = 50;
    config.treino_global = true;

    config.habilitar_similaridade = true;
    config.limite_similaridade = 0.59;

    config.habilitar_contexto = true;

    if let Err(e) = executar_treino_completo(config) {
        eprintln!("❌ treino falhou: {}", e);
        process::exit(1);
    }

    // ✅ Fallback: se o core ainda salvar fixo em src/data/mind.bin,
    // copia pro TD_MIND_PATH (que é o que o Go precisa pra download).
    let default_out = "src/data/mind.bin";

    let mind_ok = file_size(&mind_path).unwrap_or(0) > 0;
    if !mind_ok {
        let def_ok = file_size(default_out).unwrap_or(0) > 0;
        if def_ok {
            if let Some(parent) = Path::new(&mind_path).parent() {
                let _ = fs::create_dir_all(parent);
            }
            fs::copy(default_out, &mind_path).map_err(|e| {
                format!(
                    "falha copiando {} -> {}: {}",
                    default_out, mind_path, e
                )
            }).unwrap_or_else(|msg| {
                eprintln!("❌ {}", msg);
                process::exit(1);
            });

            println!("✅ fallback: copiei {} -> {}", default_out, mind_path);
        } else {
            eprintln!("❌ mind.bin não existe nem em {} nem em {}", mind_path, default_out);
            process::exit(1);
        }
    }

    println!("✅ mind.bin final: {}", mind_path);
}
