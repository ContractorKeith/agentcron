# 2. Long-term memory is mdkb, not a built-in store

agentcron hands each run mdkb's MCP server and a per-job note `agentcron/<job>` in a shared knowledge base instead of shipping its own vector store. mdkb already does Markdown files, FTS5, opt-in embeddings and git-committed writes; the trade-off is an optional runtime dependency, so memory is off with a doctor warning when mdkb is missing.
