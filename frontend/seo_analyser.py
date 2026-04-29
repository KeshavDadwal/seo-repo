def run_analysis_job(job_id, domain, max_pages=50):
    """Delegate crawling to Go backend for concurrent processing."""
    import urllib.request
    import urllib.error
    
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