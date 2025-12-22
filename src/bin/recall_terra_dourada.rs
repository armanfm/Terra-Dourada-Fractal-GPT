use std::env;
use std::path::Path;

use terra_dourada_gpt::persistencia::load_binary;
use terra_dourada_gpt::respond;

fn main() {
    let args: Vec<String> = env::args().collect();

    if args.len() < 2 {
        return; // silêncio total
    }

    let pergunta = args[1..].join(" ").trim().to_string();
    if pergunta.is_empty() {
        return;
    }

    let mind_path = env::var("TD_MIND_PATH")
        .unwrap_or_else(|_| "data/mind.bin".to_string());

    if !Path::new(&mind_path).exists() {
        eprintln!("ERRO: mind.bin não encontrado em {}", mind_path);
        return;
    }

    let memory = load_binary(&mind_path);

    if memory.banco.is_empty() {
        eprintln!("ERRO: mind.bin vazio");
        return;
    }

    let resposta = respond(&memory, &pergunta);
    println!("{}", resposta);
}
