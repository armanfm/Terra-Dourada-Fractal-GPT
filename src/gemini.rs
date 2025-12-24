import os
import subprocess
import google.generativeai as genai
from google.generativeai.types import FunctionDeclaration, Tool

# 🔑 CONFIGURAÇÃO
# Coloque sua API Key aqui ou em variável de ambiente
os.environ["GOOGLE_API_KEY"] = ""
genai.configure(api_key=os.environ["GOOGLE_API_KEY"])

# 👇 A MÁGICA: Função que chama seu RUST
def consultar_memoria_soberana(pergunta: str):
    """
    Consulta o banco de dados determinístico (Rust) para buscar fatos soberanos.
    Use isso sempre que o usuário perguntar sobre Terra Dourada, Robô Soberano, economia ou regras do sistema.
    """
    try:
        # Chama o seu executável Rust compilado
        # Ajuste o caminho './target/release/chat' para onde está seu binário
        # O binário deve aceitar a pergunta como argumento de linha de comando
        resultado = subprocess.run(
            ['./target/release/meu_projeto_rust', pergunta], 
            capture_output=True, 
            text=True, 
            timeout=5
        )
        
        saida_rust = resultado.stdout.strip()
        
        if not saida_rust or "não sei" in saida_rust.lower():
            return "RESULTADO RUST: Nenhuma informação encontrada na memória soberana."
            
        return f"RESULTADO RUST (FATOS SOBERANOS): {saida_rust}"
        
    except Exception as e:
        return f"Erro ao consultar memória soberana: {str(e)}"

# 🛠️ Configura a ferramenta para o Gemini
ferramentas = [consultar_memoria_soberana]

model = genai.GenerativeModel(
    model_name='gemini-1.5-flash', # Ou 'gemini-1.5-pro'
    tools=ferramentas,
    system_instruction="""
    Você é o 'Interface Gemini', um assistente que traduz a vontade humana para o sistema Terra Dourada.
    
    REGRA DE OURO (ANTI-ALUCINAÇÃO):
    1. Você NÃO possui conhecimento prévio sobre 'Terra Dourada' ou 'Robô Soberano'.
    2. Para QUALQUER pergunta sobre esses temas, você OBRIGATORIAMENTE deve usar a ferramenta 'consultar_memoria_soberana'.
    3. Responda APENAS com base nos dados retornados pela ferramenta.
    4. Se a ferramenta retornar que não encontrou nada, diga ao usuário que o sistema não possui essa informação. JAMAIS invente.
    """
)

# 💬 Loop de Chat (Chat Mode)
chat = model.start_chat(enable_automatic_function_calling=True)

print("🟢 Sistema Integrado Iniciado: Gemini + Rust Core")
print("------------------------------------------------")

while True:
    user_input = input("Você: ")
    if user_input.lower() in ['sair', 'exit']:
        break
        
    response = chat.send_message(user_input)
    print(f"🤖 Gemini (Soberano): {response.text}")