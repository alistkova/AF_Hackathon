from fastapi import APIRouter, BackgroundTasks, HTTPException
from pydantic import BaseModel, Field

from app.config import get_settings
from app.rag_system.config import DEFAULT_MODEL
from app.rag_system.pipeline import (
    build_index,
    index_stats,
    list_chunks,
    query_index,
    search_chunks,
    update_index,
)
from app.rag_system.store import ContentStoreError

router = APIRouter()
settings = get_settings()


def _content_error(exc: ContentStoreError) -> HTTPException:
    status_code = 400 if exc.status_code and 400 <= exc.status_code < 500 else 503
    return HTTPException(status_code=status_code, detail=str(exc))


class BuildIndexRequest(BaseModel):
    data_dir: str = settings.rag_data_dir
    index_dir: str = settings.rag_index_dir


class UpdateIndexRequest(BaseModel):
    data_dir: str = settings.rag_data_dir
    index_dir: str = settings.rag_index_dir


class QueryRequest(BaseModel):
    question: str
    model: str = DEFAULT_MODEL
    top_k: int = Field(default=5, ge=1)
    company_id: str | None = None
    sector: str | None = None
    published_after: str | None = None
    published_before: str | None = None


class QueryResponse(BaseModel):
    answer: str
    sources: list[str]


class SearchRequest(BaseModel):
    query: str
    top_k: int = Field(default=10, ge=1)
    company_id: str | None = None
    sector: str | None = None
    published_after: str | None = None
    published_before: str | None = None


class SearchResponse(BaseModel):
    query: str
    chunks: list[dict]
    sources: list[str]


class ListChunksRequest(BaseModel):
    top_k: int = Field(default=100, ge=1)
    company_id: str | None = None
    sector: str | None = None
    published_after: str | None = None
    published_before: str | None = None


class ListChunksResponse(BaseModel):
    chunks: list[dict]
    sources: list[str]


@router.post("/build-index")
async def build_index_endpoint(payload: BuildIndexRequest, background: BackgroundTasks):
    """Build RAG index from PDFs."""
    background.add_task(build_index, payload.data_dir, payload.index_dir)
    return {"status": "started", "message": "RAG index build started in background"}


@router.post("/update-index")
async def update_index_endpoint(payload: UpdateIndexRequest, background: BackgroundTasks):
    """Incremental update for new PDFs."""
    background.add_task(update_index, payload.data_dir, payload.index_dir)
    return {"status": "started", "message": "RAG index update started in background"}


@router.post("/query", response_model=QueryResponse)
async def query(payload: QueryRequest):
    """Semantic search + LLM generation over the RAG index with optional filters."""
    try:
        result = query_index(
            payload.question,
            model=payload.model,
            k=payload.top_k,
            company_id=payload.company_id,
            sector=payload.sector,
            published_after=payload.published_after,
            published_before=payload.published_before,
        )
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ContentStoreError as exc:
        raise _content_error(exc) from exc
    return QueryResponse(answer=result["answer"], sources=result["sources"])


@router.post("/search", response_model=SearchResponse)
async def search(payload: SearchRequest):
    """Semantic search with optional filters - returns chunks WITHOUT LLM generation."""
    try:
        result = search_chunks(
            payload.query,
            k=payload.top_k,
            company_id=payload.company_id,
            sector=payload.sector,
            published_after=payload.published_after,
            published_before=payload.published_before,
        )
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ContentStoreError as exc:
        raise _content_error(exc) from exc
    return SearchResponse(**result)


@router.post("/list-chunks", response_model=ListChunksResponse)
async def list_chunks_endpoint(payload: ListChunksRequest):
    """List chunks by metadata only (no semantic query)."""
    try:
        result = list_chunks(
            k=payload.top_k,
            company_id=payload.company_id,
            sector=payload.sector,
            published_after=payload.published_after,
            published_before=payload.published_before,
        )
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ContentStoreError as exc:
        raise _content_error(exc) from exc
    return ListChunksResponse(**result)


@router.get("/stats")
async def stats(index_dir: str | None = None):
    """Show RAG index statistics."""
    try:
        return index_stats(index_dir or settings.rag_index_dir)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ContentStoreError as exc:
        raise _content_error(exc) from exc
