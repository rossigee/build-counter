FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy demo script
COPY demo_generator.py .

# Run the demo generator
CMD ["python", "demo_generator.py"]