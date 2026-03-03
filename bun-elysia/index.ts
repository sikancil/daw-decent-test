import { Elysia } from 'elysia';
import { randomUUID } from 'crypto';

// Configuration from environment variables with defaults
// For local development: PORT=3082 HOST=localhost
// For Docker: defaults to 0.0.0.0:3000 internally
const PORT = process.env.PORT || '3082';
const HOST = process.env.HOST || '0.0.0.0';

// Types
type Book = {
  id: string;
  title: string;
  author: string;
  year: number;
};

// In-Memory Storage & State
const books = new Map<string, Book>();
let authUnlocked = false; // Dynamic toggle for Level 5

const app = new Elysia()
  // Level 1: Ping
  .get('/ping', () => 'pong')

  // Level 2: Echo
  .post('/echo', ({ body }) => body)

  // Level 5: Auth Guard Token Issuer
  .post('/auth/token', () => {
    authUnlocked = true; // Unlock the auth guard for subsequent GET /books requests
    return { token: 'quest-token-xyz' };
  })

  // Grouping Book Routes with Auth Middleware
  .group('/books', (app) =>
    app
      // Level 5: Dynamic Auth Guard Middleware - protects ALL book operations after unlock
      .onBeforeHandle(({ headers, set }) => {
        if (authUnlocked) {
          const auth = headers.authorization;
          if (auth !== 'Bearer quest-token-xyz') {
            set.status = 401;
            return { error: 'Unauthorized' };
          }
        }
      })
      // Level 3 & 6: Read Books, Search, Paginate
      .get('/', ({ query }) => {
        // Level 6: Search & Filter
        const authorQuery = query.author?.toLowerCase();
        let results = Array.from(books.values());

        if (authorQuery) {
          results = results.filter((b) => b.author.toLowerCase().includes(authorQuery));
        }

        // Level 6: Pagination with proper bounds checking (min 1, max 100)
        const page = Math.max(1, parseInt((query.page as string) || '1') || 1);
        const limit = Math.max(1, Math.min(100, parseInt((query.limit as string) || '10') || 10));
        const start = (page - 1) * limit;
        const end = start + limit;

        return results.slice(start, end);
      })

      // Level 3 & 7: Create Book
      .post('/', ({ body, set }) => {
        const payload = body as Partial<Book>;

        // Level 7: Error handling for invalid payload
        if (!payload || typeof payload !== 'object' || !payload.title || !payload.author) {
          set.status = 400;
          return { error: 'Invalid schema or missing fields' };
        }

        // Validate title and author are strings
        if (typeof payload.title !== 'string' || typeof payload.author !== 'string') {
          set.status = 400;
          return { error: 'Invalid schema or missing fields' };
        }

        // Validate year is a number if provided
        if (payload.year !== undefined && typeof payload.year !== 'number') {
          set.status = 400;
          return { error: 'Invalid schema or missing fields' };
        }

        const newBook: Book = {
          id: randomUUID(),
          title: payload.title,
          author: payload.author,
          // Use nullish coalescing to handle 0 vs undefined correctly
          // Default to current year (2026) only if year is undefined/null
          year: payload.year ?? new Date().getFullYear(),
        };

        books.set(newBook.id, newBook);
        set.status = 201;
        return newBook;
      })

      // Level 3 & 7: Read Single Book
      .get('/:id', ({ params: { id }, set }) => {
        const book = books.get(id);
        if (!book) {
          set.status = 404;
          return { error: 'Not found' };
        }
        return book;
      })

      // Level 4: Update Book
      .put('/:id', ({ params: { id }, body, set }) => {
        if (!books.has(id)) {
          set.status = 404;
          return { error: 'Not found' };
        }

        const payload = body as Partial<Book>;

        // Validate payload types if provided
        if (payload.title !== undefined && typeof payload.title !== 'string') {
          set.status = 400;
          return { error: 'Invalid schema' };
        }
        if (payload.author !== undefined && typeof payload.author !== 'string') {
          set.status = 400;
          return { error: 'Invalid schema' };
        }
        if (payload.year !== undefined && typeof payload.year !== 'number') {
          set.status = 400;
          return { error: 'Invalid schema' };
        }

        const existing = books.get(id)!;
        const updatedBook: Book = {
          ...existing,
          // Only include fields that were provided in payload
          title: payload.title ?? existing.title,
          author: payload.author ?? existing.author,
          // Use nullish coalescing - don't override with 0 if year is 0
          // Only use existing year if payload.year is undefined/null
          year: payload.year ?? existing.year,
          id, // Prevent ID mutation
        };

        books.set(id, updatedBook);
        return updatedBook;
      })

      // Level 4: Delete Book
      .delete('/:id', ({ params: { id }, set }) => {
        if (!books.has(id)) {
          set.status = 404;
          return { error: 'Not found' };
        }

        books.delete(id);
        set.status = 204;
        return '';
      })
  )
  .listen({
    port: parseInt(PORT),
    hostname: HOST
  });

console.log(`🦊 Elysia is running at ${HOST}:${PORT}`);
