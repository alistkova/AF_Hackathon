import pickle
from pathlib import Path
from typing import Any

from langchain_community.vectorstores import FAISS
from langchain_community.vectorstores.utils import DistanceStrategy
from langchain_core.documents import Document
from langchain_core.embeddings import Embeddings

from .config import EMBEDDING_MODEL, vector_store_path


class TfidfEmbeddings(Embeddings):
    """Wrapper for sklearn TfidfVectorizer to match LangChain embeddings interface."""

    def __init__(self):
        from sklearn.feature_extraction.text import TfidfVectorizer

        self.vectorizer = TfidfVectorizer(stop_words=None, min_df=1)
        self._fitted = False

    def __call__(self, text: str) -> list[float]:
        return self.embed_query(text)

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        if not self._fitted:
            self.vectorizer.fit(texts)
            self._fitted = True
        return self.vectorizer.transform(texts).toarray().tolist()

    def embed_query(self, text: str) -> list[float]:
        if not self._fitted:
            raise RuntimeError("Vectorizer not fitted. Call embed_documents first.")
        return self.vectorizer.transform([text]).toarray()[0].tolist()

    def save_vectorizer(self, path: str):
        """Save fitted vectorizer to disk."""
        with open(path, "wb") as f:
            pickle.dump(self.vectorizer, f)

    @classmethod
    def load_vectorizer(cls, path: str):
        """Load fitted vectorizer from disk."""
        with open(path, "rb") as f:
            vectorizer = pickle.load(f)
        instance = cls()
        instance.vectorizer = vectorizer
        instance._fitted = True
        return instance


_embedding_models: dict[str, Any] = {}


def get_embeddings(index_dir: str | Path | None = None):
    """Get or create the embedding model for an index directory."""
    store_path = vector_store_path(index_dir)
    key = str(store_path)
    if key not in _embedding_models:
        if EMBEDDING_MODEL == "tfidf":
            vectorizer_path = store_path / "tfidf_vectorizer.pkl"
            if vectorizer_path.exists():
                _embedding_models[key] = TfidfEmbeddings.load_vectorizer(str(vectorizer_path))
            else:
                _embedding_models[key] = TfidfEmbeddings()
        else:
            try:
                from langchain_huggingface import HuggingFaceEmbeddings
            except ImportError as exc:
                raise RuntimeError(
                    "HuggingFace embeddings require the 'hf' extra: uv sync --extra hf"
                ) from exc

            _embedding_models[key] = HuggingFaceEmbeddings(
                model_name=EMBEDDING_MODEL,
                model_kwargs={"device": "cpu"},
                encode_kwargs={"normalize_embeddings": True},
            )
    return _embedding_models[key]


def create_vector_store(chunks: list[Document], index_dir: str | Path | None = None) -> FAISS:
    """Create a new FAISS vector store from chunks."""
    if not chunks:
        raise ValueError("Cannot create a vector store from an empty document list")
    store_path = vector_store_path(index_dir)
    store_path.mkdir(parents=True, exist_ok=True)
    if EMBEDDING_MODEL == "tfidf":
        embeddings = TfidfEmbeddings()
    else:
        embeddings = get_embeddings(index_dir)
    texts = [c.page_content for c in chunks]
    metadatas = [c.metadata for c in chunks]
    vector_store = FAISS.from_texts(
        texts,
        embeddings,
        metadatas=metadatas,
        distance_strategy=DistanceStrategy.MAX_INNER_PRODUCT,
    )
    if hasattr(embeddings, "save_vectorizer"):
        embeddings.save_vectorizer(str(store_path / "tfidf_vectorizer.pkl"))
    _embedding_models[str(store_path)] = embeddings
    return vector_store


def load_vector_store(index_dir: str | Path | None = None) -> FAISS | None:
    """Load existing FAISS vector store."""
    store_path = vector_store_path(index_dir)
    if not store_path.exists():
        return None
    embeddings = get_embeddings(index_dir)
    return FAISS.load_local(str(store_path), embeddings, allow_dangerous_deserialization=True)


def save_vector_store(vector_store: FAISS, index_dir: str | Path | None = None) -> None:
    """Save FAISS vector store to disk."""
    store_path = vector_store_path(index_dir)
    store_path.mkdir(parents=True, exist_ok=True)
    vector_store.save_local(str(store_path))


def search(
    vector_store: FAISS, query: str, k: int = 5, score_threshold: float = 0.0
) -> list[Document]:
    """Search the vector store for relevant chunks."""
    results = vector_store.similarity_search_with_score(query, k=k)
    return [doc for doc, score in results if score >= score_threshold]
