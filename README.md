# GoAPI — Async RAG Chat & MCP API

A Go REST API for RAG chat, document ingestion, and MCP tool use.
Clients submit jobs and poll for results via an async worker pool.

Built specifically for my workplace, 
where applications operate under spotty network availability. 
This is why it uses a polling architecture instead of streaming.

Go was chosen for cross-platform support (Linux, Windows, mobile), its standard library, and native concurrency with channels and goroutines. Frameworks like LangChain simplify development, but since this is for production use at work, the decision was to rely on the standard library as much as possible to minimize external dependencies.

The architecture avoids vendor lock in. 
Embedders, LLM providers, and vector DBs are all behind opaque interfaces and can be swapped with a config change. 
LLM support includes Gemini, Claude, OpenAI, and OpenRouter.
New LLM Providers, Embedders and Vector DB's just need implement and interface and the whole system works without any interruptions.

An offline mode is in progress: sqlite-vec for vectors, Ollama for LLM/embeddings, and an in-memory store to replace Redis.

## How It Works

```
POST /chat → Job queued → Worker picks it up → RAG pipeline → Poll GET /status/{id}
```

1. Client submits a question
2. API returns a job ID immediately (202)
3. A worker processes the query through the RAG pipeline:
   - Embed the query → check semantic cache → search vector DB → generate answer via LLM
4. Client polls until the job completes

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│     API     │────→│  Job Channel │────→│ Worker Pool │
|  (config)   │     |  (buffered)  │     │ (1-10 auto) │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                │
                    ┌───────────────────────────┘
                    ▼
┌──────────────────────────────────────────────────────┐
│                    RAG Service                       │
│                                                      │
│  Embed Query → Cache Check → Vector Search → LLM     │
│  (Google)      (Qdrant)      (Qdrant)       (Multi)  │
└──────────────────────────────────────────────────────┘
```

**Worker Pool:** Starts with 1 worker, auto-scales up to 10 based on queue depth, idle workers retire after 1 minute.

**MCP Integration:** A separate `/mcp` endpoint runs an agentic tool-use loop 
Then LLM decides which tools to call via MCP protocol, using in-memory transport. Tools include `search_knowledge_base` (RAG queries) and external API integrations. New tools can be added by registering them in `mcpServer.go`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/chat` | Submit a chat query (returns job ID) |
| `GET` | `/status/{id}` | Poll job status |
| `POST` | `/ingest` | Upload PDF/DOCX/TXT for RAG ingestion |
| `POST` | `/mcp` | Stateless MCP query with tool use |
| `GET` | `/mcp/status/{id}` | Poll MCP job status |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/swagger/*` | API documentation |

## LLM Providers

Swappable via `LLM_PROVIDER` env var:

- `openrouter` , OpenRouter (default)
- `gemini` , Google Gemini
- `claude` , Anthropic Claude
- `openai` , OpenAI

All providers implement the same `llm.Provider` interface.

## Pluggable Components

The RAG pipeline uses opaque interfaces (public interface, private implementation) for its core dependencies. You can swap any of these without touching the rest of the codebase:

- **Embedder** , currently Google Embedding API, but any implementation of the `Embedder` interface works (OpenAI, Cohere, local models, etc.)
- **Vector DB** , currently Qdrant, but any implementation of the `DataProcessor` interface works (Pinecone, Weaviate, Milvus, pgvector, etc.)
- **LLM Provider** , currently supports Gemini/Claude/OpenAI/OpenRouter via the `Provider` interface
- **Data Stores** , Redis by default with automatic in-memory fallback, both implement the same `JobStore` and `MessageStore` interfaces

Each component is injected at startup via constructor. To add a new vector DB, for example, just implement the interface and pass it into `NewService()`.

## Project Structure

```
cmd/api/main.go              # Entry point
internal/
  handlers/                  # HTTP request handlers
  rag/                       # RAG pipeline (embed, search, generate)
  rag/vectorDB/qdrantDB/     # Qdrant vector database client
  worker/                    # Worker pool with auto-scaling
  job/                       # Job lifecycle management
  llm/                       # LLM provider abstraction
  llm/gemini/                # Gemini implementation
  llm/claude/                # Claude implementation
  llm/openaiModels/          # OpenAI implementation
  llm/openRouter/            # OpenRouter implementation
  mcpImpl/                   # MCP server, client, and tool-use loop
  data/store/                # Redis & in-memory job/message stores
  middleware/                # Auth, rate limiting, tracing
  metrics/                   # Prometheus counters, gauges, histograms
  config/                    # Environment variables & constants
pkg/logger_i/                # Structured logger wrapper
k6/                          # Load testing scripts
```

## Running Locally

### Prerequisites
- Go 1.25+
- Docker & Docker Compose

### Start

```bash
docker-compose up --build
```

This starts:
- **API** on `:3000`
- **Redis** on `:6379`
- **Qdrant** on `:6333` (HTTP) / `:6334` (gRPC)

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_PROVIDER` | `openrouter` | LLM provider to use |
| `LLM_MODEL_NAME` | `openrouter/auto` | Model name |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis address |
| `QDRANT_HOST` | `localhost` | Qdrant host |
| `QDRANT_PORT` | `6334` | Qdrant gRPC port |

## Testing

```bash
# Unit tests
go test ./...

# Load test (requires k6)
k6 run k6/script.js
```

## Observability

- **Metrics:** Prometheus endpoint at `/metrics`
  - `http_requests_total` — request count by path and status
  - `active_worker_count` — current workers
  - `count_jobs_in_queue` — pending jobs
  - `process_request_duration_seconds` — request latency histogram
  - `dependency_latency_seconds` — external service latencies
- **Tracing:** Every request gets a unique TraceID injected via middleware
- **Logging:** Structured JSON logs (prod) or text logs (dev)

**Note:** The document ingestion pipeline is not production ready and has known bugs. Especially with processing EU pdfs with special characters.
I am considering moving the ingestion system to python.
