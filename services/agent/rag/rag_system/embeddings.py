from langchain_huggingface import HuggingFaceEmbeddings
from langchain_community.vectorstores import FAISS
from langchain_core.documents import Document

import os
import pickle
from pathlib import Path

from .config import EMBEDDING_MODEL, VECTOR_STORE_PATH


class TfidfEmbeddings:
    """Wrapper for sklearn TfidfVectorizer to match LangChain embeddings interface."""
    def __init__(self):
        from sklearn.feature_extraction.text import TfidfVectorizer
        # Russian stop words + no stop words for better coverage
        self.vectorizer = TfidfVectorizer(stop_words=None, min_df=1, max_df=0.95)
        self._fitted = False
    
    def __call__(self, text: str) -> list[float]:
        return self.embed_query(text)
    
    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        self.vectorizer.fit(texts)
        self._fitted = True
        return self.vectorizer.transform(texts).toarray().tolist()
    
    def embed_query(self, text: str) -> list[float]:
        if not self._fitted:
            raise RuntimeError('Vectorizer not fitted. Call embed_documents first.')
        return self.vectorizer.transform([text]).toarray()[0].tolist()
    
    def save_vectorizer(self, path: str):
        """Save fitted vectorizer to disk."""
        with open(path, 'wb') as f:
            pickle.dump(self.vectorizer, f)
    
    @classmethod
    def load_vectorizer(cls, path: str):
        """Load fitted vectorizer from disk."""
        with open(path, 'rb') as f:
            vectorizer = pickle.load(f)
        instance = cls()
        instance.vectorizer = vectorizer
        instance._fitted = True
        return instance


_embedding_model = None


def get_embeddings():
    """Get or create the embedding model (singleton)."""
    global _embedding_model
    if _embedding_model is None:
        if EMBEDDING_MODEL == "tfidf":
            # Try to load existing fitted vectorizer
            vectorizer_path = Path(VECTOR_STORE_PATH) / "tfidf_vectorizer.pkl"
            if vectorizer_path.exists():
                _embedding_model = TfidfEmbeddings.load_vectorizer(str(vectorizer_path))
            else:
                _embedding_model = TfidfEmbeddings()
        else:
            _embedding_model = HuggingFaceEmbeddings(
                model_name=EMBEDDING_MODEL,
                model_kwargs={"device": "cpu"},
                encode_kwargs={"normalize_embeddings": True}
            )
    return _embedding_model


def create_vector_store(chunks: list[Document]) -> FAISS:
    """Create a new FAISS vector store from chunks."""
    Path(VECTOR_STORE_PATH).mkdir(parents=True, exist_ok=True)
    embeddings = get_embeddings()
    texts = [c.page_content for c in chunks]
    metadatas = [c.metadata for c in chunks]
    vector_store = FAISS.from_texts(texts, embeddings, metadatas=metadatas)
    # Save fitted vectorizer
    if hasattr(embeddings, 'save_vectorizer'):
        embeddings.save_vectorizer(str(Path(VECTOR_STORE_PATH) / "tfidf_vectorizer.pkl"))
    return vector_store


def load_vector_store() -> FAISS | None:
    """Load existing FAISS vector store."""
    if not os.path.exists(VECTOR_STORE_PATH):
        return None
    embeddings = get_embeddings()
    return FAISS.load_local(VECTOR_STORE_PATH, embeddings, allow_dangerous_deserialization=True)


def save_vector_store(vector_store: FAISS):
    """Save FAISS vector store to disk."""
    Path(VECTOR_STORE_PATH).mkdir(parents=True, exist_ok=True)
    vector_store.save_local(VECTOR_STORE_PATH)


def add_chunks_to_store(vector_store: FAISS, chunks: list[Document]):
    """Add new chunks to an existing vector store."""
    if not chunks:
        return
    texts = [c.page_content for c in chunks]
    metadatas = [c.metadata for c in chunks]
    vector_store.add_texts(texts, metadatas=metadatas)
    # Update saved vectorizer
    if hasattr(vector_store.embedding_function, 'save_vectorizer'):
        vector_store.embedding_function.save_vectorizer(str(Path(VECTOR_STORE_PATH) / "tfidf_vectorizer.pkl"))


def search(vector_store: FAISS, query: str, k: int = 5, score_threshold: float = 0.5) -> list[Document]:
    """Search the vector store for relevant chunks."""
    results = vector_store.similarity_search_with_score(query, k=k)
    filtered = [doc for doc, score in results if score >= score_threshold]
    return filtered
