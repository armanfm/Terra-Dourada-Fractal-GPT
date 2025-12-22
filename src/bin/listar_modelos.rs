use reqwest::Client;
use serde_json::Value;
use std::env;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Lê a chave da API do ambiente
    let api_key = env::var("GEMINI_API_KEY")
        .expect("ERRO: Defina a variável de ambiente GEMINI_API_KEY");

    let url = format!(
        "https://generativelanguage.googleapis.com/v1beta/models?key={}",
        api_key
    );

    println!("🔍 Listando modelos disponíveis para esta chave...\n");

    let client = Client::new();
    let res = client.get(&url).send().await?;

    if !res.status().is_success() {
        println!("❌ Erro HTTP: {}", res.status());
        println!("{}", res.text().await?);
        return Ok(());
    }

    let body: Value = res.json().await?;

    if let Some(models) = body["models"].as_array() {
        for model in models {
            let name = model["name"].as_str().unwrap_or("");
            let display = model["displayName"].as_str().unwrap_or("");
            let methods = model["supportedGenerationMethods"].as_array();

            // Só mostra modelos que aceitam generateContent (chat)
            let suporta_chat = methods
                .map(|m| m.iter().any(|x| x.as_str() == Some("generateContent")))
                .unwrap_or(false);

            if suporta_chat {
                println!("🔹 Nome para usar no código:");
                println!("    {}", name.replace("models/", ""));
                println!("   Apelido: {}", display);
                println!("--------------------------------------------");
            }
        }
    } else {
        println!("⚠️ Nenhum modelo encontrado.");
        println!("{:#?}", body);
    }

    Ok(())
}
