import os
import uuid
from datetime import datetime
from typing import Optional, Dict
import uvicorn
from fastapi import FastAPI, Request, Response, HTTPException, status
from fastapi.responses import JSONResponse
from fastapi.exceptions import RequestValidationError
from pydantic import BaseModel, Field, field_validator

# Configuration from environment variables with defaults
# For local development: PORT=3000 HOST=localhost
# For Docker: defaults to 0.0.0.0:3000 internally
PORT = int(os.getenv("PORT", "3000"))
HOST = os.getenv("HOST", "0.0.0.0")

app = FastAPI(docs_url=None, redoc_url=None)

# State & In-Memory Storage
books_db: Dict[str, dict] = {}
auth_unlocked: bool = False

# Pydantic Schema
class BookPayload(BaseModel):
    title: str = Field(..., min_length=1, description="Book title")
    author: str = Field(..., min_length=1, description="Book author")
    year: Optional[int] = Field(None, ge=0, le=9999, description="Publication year")

    @field_validator('title', 'author')
    @classmethod
    def validate_not_empty(cls, v: str) -> str:
        if not v or not v.strip():
            raise ValueError('cannot be empty')
        return v

# Level 7: Override FastAPI's default 422 error to 400 Bad Request to strictly pass the tests
@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    return JSONResponse(
        status_code=status.HTTP_400_BAD_REQUEST,
        content={"error": "Invalid schema or missing fields"}
    )

# Level 5: Dynamic Auth Guard Middleware - protects ALL book operations after unlock
@app.middleware("http")
async def auth_middleware(request: Request, call_next):
    """Dynamic auth guard that activates after POST /auth/token is called."""
    # Only check auth for book routes after unlock
    if auth_unlocked and request.url.path.startswith("/books"):
        auth_header = request.headers.get("Authorization")
        if auth_header != "Bearer quest-token-xyz":
            return JSONResponse(
                status_code=status.HTTP_401_UNAUTHORIZED,
                content={"error": "Unauthorized"}
            )
    return await call_next(request)

# Level 1: Ping
@app.get("/ping")
async def ping():
    return Response(content="pong", media_type="text/plain")

# Level 2: Echo
@app.post("/echo")
async def echo(request: Request):
    body = await request.json()
    return body

# Level 5: Auth Guard Token Issuer
@app.post("/auth/token")
async def get_token():
    global auth_unlocked
    auth_unlocked = True
    return {"token": "quest-token-xyz"}

# Level 3 & 6: Read Books, Search, Paginate
@app.get("/books")
async def get_books(
    author: Optional[str] = None,
    page: int = 1,
    limit: int = 10
):
    # Level 6: Search & Filter
    results = list(books_db.values())
    if author:
        results = [b for b in results if author.lower() in b["author"].lower()]

    # Level 6: Pagination with proper bounds checking (min 1, max 100)
    page = max(1, min(page, 1000000))  # reasonable upper bound for page
    limit = max(1, min(limit, 100))     # max 100 items per page
    start = (page - 1) * limit
    end = start + limit

    return results[start:end]

# Level 3 & 7: Create Book
@app.post("/books", status_code=status.HTTP_201_CREATED)
async def create_book(payload: BookPayload):
    book_id = str(uuid.uuid4())

    # Default year to current year (2026) if not provided
    year = payload.year if payload.year is not None else datetime.now().year

    new_book = {
        "id": book_id,
        "title": payload.title,
        "author": payload.author,
        "year": year
    }
    books_db[book_id] = new_book
    return new_book

# Level 3 & 7: Read Single Book
@app.get("/books/{book_id}")
async def get_book(book_id: str):
    if book_id not in books_db:
        raise HTTPException(status_code=404, detail="Not found")
    return books_db[book_id]

# Level 4: Update Book
@app.put("/books/{book_id}")
async def update_book(book_id: str, payload: BookPayload):
    if book_id not in books_db:
        raise HTTPException(status_code=404, detail="Not found")

    existing_book = books_db[book_id]

    # Only update year if provided, otherwise keep existing value
    year = payload.year if payload.year is not None else existing_book.get("year", datetime.now().year)

    updated_book = {
        "id": book_id,
        "title": payload.title,
        "author": payload.author,
        "year": year
    }
    books_db[book_id] = updated_book
    return updated_book

# Level 4: Delete Book
@app.delete("/books/{book_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_book(book_id: str):
    if book_id not in books_db:
        raise HTTPException(status_code=404, detail="Not found")
    del books_db[book_id]
    return Response(status_code=status.HTTP_204_NO_CONTENT)


# Entry point for running the server
if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host=HOST,
        port=PORT,
        loop="uvloop",
        log_level="info"
    )
