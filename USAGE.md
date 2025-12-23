# PropLeads Usage Guide

## Two Ways to Run PropLeads

### 1. CLI Mode (Command Line - No Frontend)

Run the scraper directly from the command line:

```bash
# County scraping with auto-scaling workers
./scraper --county newhanover

# SOS scraping only
./scraper --sos-only

# Custom worker count
./scraper --sos-only --workers 10

# Data processing only
./scraper --process-only

# Reconciliation only
./scraper --reconcile-only
```

**Use CLI when:**
- Running on a server/cron job
- No UI needed
- Automation/scripts

---

### 2. API + Frontend Mode (Web Interface)

Run the backend API server and connect the frontend:

#### Step 1: Start the Backend API Server
```bash
./server
```
The server will start on `http://localhost:8080`

#### Step 2: Start the Frontend
```bash
cd propleads-connect
npm install  # First time only
npm run dev
```
The frontend will start on `http://localhost:5173` (or similar)

#### Step 3: Use the Web Interface
1. Open browser to `http://localhost:5173`
2. Upload a CSV file with PIDs (one per line)
3. Select county (newhanover, brunswick, or pender)
4. Click "Start Scraping"
5. Watch real-time progress
6. Download results when complete

**Use API + Frontend when:**
- Want a visual interface
- Multiple users need access
- Need to monitor progress in real-time
- Prefer drag-and-drop file uploads

---

## File Locations

### Input Files:
- `data/input/pids.csv` - Parcel IDs to scrape (CLI mode)

### Output Files:
- `data/output/parcel_results.csv` - Property data from county
- `data/output/sos_results.csv` - Business officials from NC SOS
- `data/output/unified_results.csv` - Merged and processed data
- `data/output/names.csv` - Extracted individual names
- `data/output/names_for_whitepages.csv` - Names for WhitePages lookup

When using API mode, files are named with job IDs: `parcel_results_{jobId}.csv`

---

## Performance

### Auto-Scaling Workers:
- 1-9 businesses: 3 workers
- 10-99 businesses: 5 workers
- 100-199 businesses: 8 workers
- 200+ businesses: 10 workers

### Estimated Times:
- 50 businesses: ~4 minutes
- 100 businesses: ~5 minutes
- 200 businesses: ~8 minutes
- 500 businesses: ~20 minutes

---

## Requirements

### For CLI Mode:
- Chrome/Chromium installed
- xvfb installed (`sudo apt install -y xvfb`)
- Python 3.12+ with seleniumbase (`pip3 install seleniumbase`)

### For API + Frontend Mode:
All of the above, plus:
- Node.js 18+ (`nvm install 18`)
- npm or bun

---

## Both Modes Work Simultaneously!

You can:
- Run CLI scrapes while the API server is running
- Have the frontend submit jobs via API
- Mix and match as needed

The CLI and API are completely independent.
