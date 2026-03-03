FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY main.py .

EXPOSE 3000

# Run Uvicorn with uvloop for maximum performance
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "3000", "--loop", "uvloop"]
