"""
API Quest - Python FastAPI Tests

Unit and E2E tests for the Python FastAPI implementation.

To run tests:
    uv run pytest test_main.py -v

Or with the Makefile:
    make test-py
"""

import pytest
from starlette.testclient import TestClient
from main import app, books_db, auth_unlocked

# Reset global state before each test
@pytest.fixture(autouse=True)
def reset_state():
    """Reset global state before each test."""
    # Clear books and reset auth - need to modify the module-level variable
    books_db.clear()
    # Import the module and reset auth_unlocked
    import main
    main.auth_unlocked = False
    yield
    # Cleanup after test
    books_db.clear()
    main.auth_unlocked = False


# Test client
@pytest.fixture
def client():
    """Create a test client."""
    return TestClient(app)


# =============================================================================
# Level 1: Ping
# =============================================================================

def test_level1_ping_returns_pong(client):
    """Test GET /ping returns 'pong' with text/plain content type."""
    response = client.get("/ping")
    assert response.status_code == 200
    assert response.text == "pong"
    assert response.headers["content-type"] == "text/plain; charset=utf-8"


# =============================================================================
# Level 2: Echo
# =============================================================================

def test_level2_echo_returns_json_body(client):
    """Test POST /echo returns the exact JSON body."""
    payload = {"message": "hello world", "foo": "bar"}
    response = client.post("/echo", json=payload)
    assert response.status_code == 200
    assert response.json() == payload


# =============================================================================
# Level 3: Create & Read Books
# =============================================================================

def test_level3_create_book_returns_201(client):
    """Test POST /books creates a book and returns 201."""
    payload = {
        "title": "Test Book",
        "author": "Test Author",
        "year": 2024
    }
    response = client.post("/books", json=payload)
    assert response.status_code == 201
    data = response.json()
    assert data["title"] == "Test Book"
    assert data["author"] == "Test Author"
    assert data["year"] == 2024
    assert "id" in data


def test_level3_get_all_books_returns_array(client):
    """Test GET /books returns array of books."""
    # Create a test book first
    client.post("/books", json={
        "title": "Sample Book",
        "author": "Sample Author",
        "year": 2024
    })

    response = client.get("/books")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) >= 1


def test_level3_get_single_book_returns_book(client):
    """Test GET /books/:id returns the specific book."""
    # Create a test book
    create_response = client.post("/books", json={
        "title": "Single Book",
        "author": "Single Author",
        "year": 2024
    })
    book_id = create_response.json()["id"]

    # Get the book
    response = client.get(f"/books/{book_id}")
    assert response.status_code == 200
    data = response.json()
    assert data["title"] == "Single Book"
    assert data["id"] == book_id


# =============================================================================
# Level 4: Update & Delete Books
# =============================================================================

def test_level4_update_book_returns_200(client):
    """Test PUT /books/:id updates a book."""
    # Create a test book
    create_response = client.post("/books", json={
        "title": "Original Title",
        "author": "Original Author",
        "year": 2024
    })
    book_id = create_response.json()["id"]

    # Update the book
    update_payload = {
        "title": "Updated Title",
        "author": "Updated Author",
        "year": 2026
    }
    response = client.put(f"/books/{book_id}", json=update_payload)
    assert response.status_code == 200
    data = response.json()
    assert data["title"] == "Updated Title"
    assert data["author"] == "Updated Author"
    assert data["year"] == 2026
    assert data["id"] == book_id  # ID should not change


def test_level4_delete_book_returns_204(client):
    """Test DELETE /books/:id removes a book and returns 204."""
    # Create a test book
    create_response = client.post("/books", json={
        "title": "Delete Me",
        "author": "Delete Author",
        "year": 2024
    })
    book_id = create_response.json()["id"]

    # Delete the book
    response = client.delete(f"/books/{book_id}")
    assert response.status_code == 204
    assert response.text == ""

    # Verify it's deleted
    get_response = client.get(f"/books/{book_id}")
    assert get_response.status_code == 404


# =============================================================================
# Level 5: Auth Guard
# =============================================================================

def test_level5_auth_token_returns_token(client):
    """Test POST /auth/token returns a valid token."""
    response = client.post("/auth/token")
    assert response.status_code == 200
    data = response.json()
    assert "token" in data
    assert data["token"] == "quest-token-xyz"


# =============================================================================
# Level 6: Search & Pagination
# =============================================================================

def test_level6_search_by_author(client):
    """Test GET /books?author=X filters by author."""
    # Create test books
    client.post("/books", json={"title": "Book 1", "author": "Alice", "year": 2024})
    client.post("/books", json={"title": "Book 2", "author": "Bob", "year": 2024})
    client.post("/books", json={"title": "Book 3", "author": "Alice Smith", "year": 2024})

    # Search for Alice
    response = client.get("/books?author=Alice")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    # All returned books should have "alice" in author (case insensitive)
    for book in data:
        assert "alice" in book["author"].lower()


def test_level6_pagination(client):
    """Test GET /books?page=X&limit=Y paginates results."""
    # Create multiple books
    for i in range(5):
        client.post("/books", json={
            "title": f"Book {i}",
            "author": f"Author {i}",
            "year": 2024
        })

    # Get first page with limit 2
    response = client.get("/books?page=1&limit=2")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) <= 2


# =============================================================================
# Level 7: Error Handling
# =============================================================================

def test_level7_invalid_schema_returns_400(client):
    """Test POST /books with invalid schema returns 400."""
    # Missing required field 'author'
    invalid_payload = {"title": "No Author"}
    response = client.post("/books", json=invalid_payload)
    assert response.status_code == 400
    data = response.json()
    assert "error" in data


def test_level7_not_found_returns_404(client):
    """Test GET /books/:id with non-existent ID returns 404."""
    response = client.get("/books/non-existent-uuid-12345")
    assert response.status_code == 404
    data = response.json()
    assert "detail" in data or "error" in data


# =============================================================================
# E2E: Full Workflow Test
# =============================================================================

def test_e2e_full_workflow(client):
    """Test the complete API workflow end-to-end."""
    # 1. Ping
    response = client.get("/ping")
    assert response.status_code == 200
    assert response.text == "pong"

    # 2. Echo
    response = client.post("/echo", json={"test": "data"})
    assert response.status_code == 200
    assert response.json()["test"] == "data"

    # 3. Create book
    response = client.post("/books", json={
        "title": "E2E Test Book",
        "author": "E2E Author",
        "year": 2024
    })
    assert response.status_code == 201
    book_id = response.json()["id"]

    # 4. Get all books
    response = client.get("/books")
    assert response.status_code == 200
    assert isinstance(response.json(), list)

    # 5. Get single book
    response = client.get(f"/books/{book_id}")
    assert response.status_code == 200
    assert response.json()["title"] == "E2E Test Book"

    # 6. Update book
    response = client.put(f"/books/{book_id}", json={
        "title": "Updated E2E Book",
        "author": "E2E Author",
        "year": 2026
    })
    assert response.status_code == 200
    assert response.json()["title"] == "Updated E2E Book"

    # 7. Delete book
    response = client.delete(f"/books/{book_id}")
    assert response.status_code == 204

    # 8. Verify deletion
    response = client.get(f"/books/{book_id}")
    assert response.status_code == 404


# =============================================================================
# Edge Cases
# =============================================================================

def test_update_nonexistent_book_returns_404(client):
    """Test PUT /books/:id with non-existent book returns 404."""
    response = client.put("/books/non-existent-id", json={
        "title": "Won't Work",
        "author": "Nope",
        "year": 2024
    })
    assert response.status_code == 404


def test_delete_nonexistent_book_returns_404(client):
    """Test DELETE /books/:id with non-existent book returns 404."""
    response = client.delete("/books/non-existent-id")
    assert response.status_code == 404


def test_book_with_default_year(client):
    """Test creating a book without year defaults to current year."""
    from datetime import datetime
    current_year = datetime.now().year

    response = client.post("/books", json={
        "title": "Default Year Book",
        "author": "Test Author"
    })
    assert response.status_code == 201
    # Year should default to current year
    assert response.json()["year"] == current_year
