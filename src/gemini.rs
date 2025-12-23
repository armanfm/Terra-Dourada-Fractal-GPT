import os
import subprocess
import google.generativeai as genai

API_ENV = "GOOGLE_API_KEY"   # ou "GEMINI_API_KEY" se você preferir padronizar
BIN_ENV = "TD_RUST_BIN"      # caminho pro executável Rust que responde consultas

def consultar_memoria_soberana(pergunta: str) -> str:
    bin_path = os.getenv(BIN_ENV)
    if not bin_path:
        return f"Erro: defina {BIN_ENV} com o caminho do executável Rust."

    try:
        r = subprocess.run(
            [bin_path, pergunta],
            capture_output=True,
            text=True,
            timeout=8
        )

        out = (r.stdout or "").strip()
        if not out:
            err = (r.stderr or "").strip()
            return f"RESULTADO RUST: vazio. stderr={err}" if err else "RESULTADO RUST: vazio."

        if "não sei" in out.lower() or "nao sei" in out.lower():
            return "RESULTADO RUST: Nenhuma informação encontrada na memória soberana."

        return f"RESULTADO RUST (FATOS SOBERANOS): {out}"

    except subprocess.TimeoutExpired:
        return "Erro: timeout consultando a memória soberana (Rust demorou demais)."
    except Exception as e:
        return f"Erro ao consultar memória soberana: {e}"

def main():
    api_key = os.getenv(API_ENV)
    if not api_key:
        raise SystemExit(f"ERRO: defina {API_ENV} no ambiente.")

    genai.configure(api_key=api_key)

    model = genai.GenerativeModel(
        model_name="gemini-1.5-flash",
        tools=[consultar_memoria_soberana],
        system_instruction=(
            "Você é o 'Interface Gemini', tradutor da vontade humana para o sistema Terra Dourada.\n"
            "REGRA DE OURO (ANTI-ALUCINAÇÃO):\n"
            "1) Você NÃO possui conhecimento prévio sobre Terra Dourada.\n"
            "2) Para qualquer pergunta sobre Terra Dourada / Robô Soberano / regras do sistema, "
            "você DEVE usar consultar_memoria_soberana.\n"
            "3) Responda apenas com base no retorno.\n"
            "4) Se não houver dados, diga que não há informação. Não invente.\n"
        ),
    )

    chat = model.start_chat(enable_automatic_function_calling=True)

    print("🟢 Sistema Integrado: Gemini + Rust Core")
    print(f"🔧 BIN: {os.getenv(BIN_ENV, '(não definido)')}")
    print("Digite 'exit' para sair.\n")

    while True:
        user_input = input("Você: ").strip()
        if user_input.lower() in ("exit", "sair"):
            break
        if not user_input:
            continue

        resp = chat.send_message(user_input)
        print(f"🤖 Gemini (Soberano): {resp.text}\n")

if __name__ == "__main__":
    main()
