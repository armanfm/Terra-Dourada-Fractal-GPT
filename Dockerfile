# ===============================
# STAGE 1 — build Rust (engine)
# ===============================
FROM rust:1.82 AS rust-builder

WORKDIR /app

# Copia manifestos primeiro (cache correto)
COPY Cargo.toml ./
COPY Cargo.lock ./  # se não existir localmente, pode remover esta linha

# Copia código Rust
COPY src ./src

# Compila apenas os bins necessários
RUN cargo build --release \
    --bin treino \
    --bin recall_terra_dourada


# ===============================
# STAGE 2 — build Go (server)
# ===============================
FROM golang:1.22 AS go-builder

WORKDIR /app

# Copia módulo Go primeiro
COPY go.mod go.sum* ./

# Resolve dependências
RUN go mod download

# Copia código Go
COPY server.go ./

# Compila binário Linux estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server


# ===============================
# STAGE 3 — runtime (produção)
# ===============================
FROM debian:bookworm-slim

WORKDIR /app

# Certificados TLS (Gemini / HTTPS)
RUN apt-get update \
 && apt-get install -y ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Copia binários Rust
COPY --from=rust-builder /app/target/release /app/target/release

# Copia binário Go
COPY --from=go-builder /app/server /app/server

# Diretório do mind.bin
RUN mkdir -p /app/src/data

# Variáveis de ambiente
ENV PORT=8080
ENV TD_MIND_PATH=/app/src/data/mind.bin

EXPOSE 8080

# Processo principal
CMD ["./server"]
