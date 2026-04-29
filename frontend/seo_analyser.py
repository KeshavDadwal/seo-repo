import os
import json
import time
import threading
import urllib.request
import urllib.error
import urllib.parse
from http.server import BaseHTTPRequestHandler, HTTPServer
import mimetypes

JOBS = {}
JOB_LOCK = threading.Lock()

def run_analysis_job(job_id, domain, max_pages=50):
    """Delegate crawling to Go backend for concurrent processing."""
    GO_BACKEND = os.environ.get("GO_BACKEND_URL", "http://localhost:8080")
    
    with JOB_LOCK:
        JOBS[job_id] = {
            "status": "fetching_sitemap",
            "domain": domain,
            "progress": 0,
            "total": 0,
            "sitemap_url": None,
            "urls": [],
            "results": [],
            "error": None
        }

    try:
        # Start analysis on Go backend
        req = urllib.request.Request(
            f"{GO_BACKEND}/api/analyse?domain={urllib.parse.quote(domain)}&max_pages={max_pages}",
            method="GET"
        )
        
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
            go_job_id = data["job_id"]
        
        # Poll until complete
        while True:
            time.sleep(0.8)
            status_req = urllib.request.Request(f"{GO_BACKEND}/api/status?job_id={go_job_id}")
            with urllib.request.urlopen(status_req, timeout=10) as resp:
                status_data = json.loads(resp.read())
            
            with JOB_LOCK:
                JOBS[job_id]["status"] = status_data["status"]
                JOBS[job_id]["progress"] = status_data["progress"]
                JOBS[job_id]["total"] = status_data["total"]
                JOBS[job_id]["sitemap_url"] = status_data.get("sitemap_url")
            
            if status_data["status"] in ("complete", "error"):
                break
        
        # Fetch results
        results_req = urllib.request.Request(f"{GO_BACKEND}/api/results?job_id={go_job_id}")
        with urllib.request.urlopen(results_req, timeout=60) as resp:
            final_data = json.loads(resp.read())
        
        with JOB_LOCK:
            JOBS[job_id]["results"] = final_data.get("results", [])
            JOBS[job_id]["status"] = final_data.get("status", "complete")

    except Exception as e:
        with JOB_LOCK:
            JOBS[job_id]["status"] = "error"
            JOBS[job_id]["error"] = str(e)


class SEORequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed_path = urllib.parse.urlparse(self.path)
        
        if parsed_path.path == '/':
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            try:
                with open('templates/index.html', 'rb') as f:
                    self.wfile.write(f.read())
            except FileNotFoundError:
                self.wfile.write(b"<h1>Error: index.html not found</h1>")
            return

        elif parsed_path.path == '/api/status':
            query = urllib.parse.parse_qs(parsed_path.query)
            job_id = query.get('job_id', [''])[0]
            
            with JOB_LOCK:
                job = JOBS.get(job_id)
                
            if job:
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps(job).encode())
            else:
                self.send_response(404)
                self.end_headers()
                self.wfile.write(b'{"error": "job not found"}')
            return

        elif parsed_path.path == '/api/results':
            query = urllib.parse.parse_qs(parsed_path.query)
            job_id = query.get('job_id', [''])[0]
            
            with JOB_LOCK:
                job = JOBS.get(job_id)
                
            if job and "results" in job:
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                # Return just the results array wrapped in an object, as the UI expects data.results
                self.wfile.write(json.dumps({"results": job["results"]}).encode())
            else:
                self.send_response(404)
                self.end_headers()
                self.wfile.write(b'{"error": "results not found"}')
            return


        elif parsed_path.path.startswith('/static/'):
            filepath = parsed_path.path.lstrip('/')
            try:
                with open(filepath, 'rb') as f:
                    content = f.read()
                self.send_response(200)
                mime_type, _ = mimetypes.guess_type(filepath)
                if mime_type:
                    self.send_header('Content-type', mime_type)
                self.end_headers()
                self.wfile.write(content)
            except FileNotFoundError:
                self.send_response(404)
                self.end_headers()
            return
            
        else:
            self.send_response(404)
            self.end_headers()
            return

    def do_POST(self):
        parsed_path = urllib.parse.urlparse(self.path)
        
        if parsed_path.path == '/api/analyse':
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)
            data = json.loads(post_data.decode('utf-8'))
            
            domain = data.get('domain')
            if not domain:
                self.send_response(400)
                self.end_headers()
                self.wfile.write(b'{"error": "domain required"}')
                return
            
            query = urllib.parse.parse_qs(parsed_path.query)
            max_pages_str = query.get('max_pages', ['50'])[0]
            try:
                max_pages = int(max_pages_str)
            except ValueError:
                max_pages = 50
                
            job_id = str(time.time()).replace('.', '')
            
            thread = threading.Thread(target=run_analysis_job, args=(job_id, domain, max_pages))
            thread.daemon = True
            thread.start()
            
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({"job_id": job_id}).encode())
            return
            
        else:
            self.send_response(404)
            self.end_headers()

if __name__ == '__main__':
    port = int(os.environ.get("PORT", 5000))
    server = HTTPServer(('0.0.0.0', port), SEORequestHandler)
    print(f"Starting Python server on port {port}...")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()
    print("Server stopped.")