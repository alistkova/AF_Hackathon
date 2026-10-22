"""RAG system for Russian news/newsletter PDFs with local LLM integration."""

from .config import *
from .ingest import ingest_all
from .pipeline import build_index, update_index, query_index, index_stats, get_retriever
from .generate import rag_query, rag_stream, generate_with_ollama
from .retrieve import retrieve, retrieve_with_context, Retriever
from .embeddings import create_vector_store, load_vector_store, save_vector_store, search
