FROM python:3.11-slim
WORKDIR /app
COPY . .
EXPOSE 5000
ENV GO_BACKEND_URL=http://go-crawler:8080
CMD ["python3", "seo_analyser.py"]