from fastapi import APIRouter, BackgroundTasks, HTTPException
from pydantic import BaseModel

router = APIRouter()


class BuildIndexRequest(BaseModel):
    data_dir: str = "services/agent/rag/data"
    index_dir: str = "C:/temp/alfa_rag_index"


class UpdateIndexRequest(BaseModel):
    data_dir: str = "services/agent/rag/data"
    index_dir: str = "C:/temp/alfa_rag_index"


class QueryRequest(BaseModel):
    question: str
    index_dir: str = "C:/temp/alfa_rag_index"
    top_k: int = 5


class QueryResponse(BaseModel):
    answer: str
    sources: list[str]


@router.post("/build-index")
async def build_index(payload: BuildIndexRequest, background: BackgroundTasks):
    """Build RAG index from PDFs."""
    from app.rag_system.pipeline import build_index as build_index_fn

    background.add_task(build_index_fn, payload.data_dir, payload.index_dir)
    return {"status": "started", "message": "RAG index build started in background"}


@router.post("/update-index")
async def update_index(payload: UpdateIndexRequest, background: BackgroundTasks):
    """Incremental update for new PDFs."""
    from app.rag_system.pipeline import update_index as update_index_fn

    background.add_task(update_index_fn, payload.data_dir, payload.index_dir)
    return {"status": "started", "message": "RAG index update started in background"}


@router.post("/query", response_model=QueryResponse)
async def query(payload: QueryRequest):
    """Semantic search + LLM generation over the RAG index."""
    from app.rag_system.pipeline import query_index

    try:
        answer, sources = query_index(payload.question, payload.index_dir, payload.top_k)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return QueryResponse(answer=answer, sources=sources)


@router.get("/stats")
async def stats(index_dir: str = "C:/temp/alfa_rag_index"):
    """Show RAG index statistics."""
    from app.rag_system.pipeline import index_stats

    try:
        return index_stats(index_dir)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
