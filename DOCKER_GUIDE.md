# 🐳 PropLeads Docker Deployment Guide

## Quick Start - One Command Deployment!

```bash
# Start everything (backend + frontend)
docker-compose up -d

# That's it! 🎉
```

**Access the app:**
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

---

## What Docker Provides:

✅ **Complete Environment**
- Go backend with SQLite database
- Python scrapers (SeleniumBase + Botasaurus)
- Google Chrome with xvfb (headless)
- React frontend (production build)
- nginx web server

✅ **Easy Deployment**
- Single command to start/stop
- Automatic container orchestration
- Data persistence via volumes
- Health checks included

✅ **No Manual Setup**
- No need to install Go, Python, Node.js
- No need to install Chrome, xvfb
- All dependencies included

---

## Commands:

### Start Services
```bash
# Start in background
docker-compose up -d

# Start with logs visible
docker-compose up

# Start and rebuild images
docker-compose up --build
```

### Stop Services
```bash
# Stop containers
docker-compose down

# Stop and remove volumes (⚠️  deletes database!)
docker-compose down -v
```

### View Logs
```bash
# All services
docker-compose logs -f

# Backend only
docker-compose logs -f backend

# Frontend only
docker-compose logs -f frontend
```

### Check Status
```bash
docker-compose ps
```

### Restart Services
```bash
docker-compose restart
```

---

## Architecture:

```
┌─────────────────────────────────────────┐
│  Frontend (React + nginx)               │
│  Port: 3000                             │
│  Container: propleads-frontend          │
└──────────────┬──────────────────────────┘
               │
               ↓ API calls
┌──────────────────────────────────────────┐
│  Backend (Go + Python)                   │
│  Port: 8080                              │
│  Container: propleads-backend            │
│                                          │
│  ✓ Go API server                         │
│  ✓ SeleniumBase UC Mode                  │
│  ✓ Chrome + xvfb (headless)              │
│  ✓ SQLite database                       │
└──────────────────────────────────────────┘

Volume: ./data → /app/data
  (Persists database and results)
```

---

## Data Persistence:

Your data is stored in the `./data` directory and mounted into the container:

```
./data/
  ├── propleads.db          (User database)
  ├── input/
  │   └── pids.csv         (Input PIDs)
  └── output/
      ├── parcel_results_{jobId}.csv
      ├── sos_results_{jobId}.csv
      └── unified_results_{jobId}.csv
```

**Data survives container restarts!**

---

## Configuration:

### Environment Variables

Create `.env` file in project root:

```env
# JWT Secret (CHANGE IN PRODUCTION!)
JWT_SECRET=your-super-secret-production-key-here-2024

# API URL for frontend
VITE_API_URL=http://localhost:8080/api
```

### Port Configuration

Edit `docker-compose.yml`:

```yaml
services:
  backend:
    ports:
      - "8080:8080"  # Change left side for different host port

  frontend:
    ports:
      - "3000:80"    # Change left side for different host port
```

---

## Production Deployment:

### 1. Set Production Secret
```bash
export JWT_SECRET="$(openssl rand -base64 32)"
echo "JWT_SECRET=$JWT_SECRET" > .env
```

### 2. Update API URL (if needed)
```env
# If backend is on different domain
VITE_API_URL=https://api.yourdomain.com/api
```

### 3. Build and Deploy
```bash
docker-compose up -d --build
```

### 4. Setup Reverse Proxy (Optional)

Example nginx config for domain:

```nginx
# Frontend
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# Backend API
server {
    listen 80;
    server_name api.yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## Troubleshooting:

### Container won't start
```bash
# Check logs
docker-compose logs backend
docker-compose logs frontend

# Rebuild from scratch
docker-compose down
docker-compose build --no-cache
docker-compose up
```

### Can't access frontend
- Check: http://localhost:3000
- Verify container running: `docker-compose ps`
- Check frontend logs: `docker-compose logs frontend`

### Can't access backend
- Check: http://localhost:8080/api/auth/login
- Verify container running: `docker-compose ps`
- Check backend logs: `docker-compose logs backend`

### Database issues
```bash
# Reset database (⚠️  deletes all users!)
docker-compose down
rm data/propleads.db
docker-compose up -d
```

### Chrome/scraping issues
```bash
# Check if Chrome is installed in container
docker exec propleads-backend google-chrome --version

# Check if xvfb is running
docker exec propleads-backend ps aux | grep Xvfb
```

---

## Development vs Production:

### Development (local)
```bash
# Backend
./server

# Frontend
cd propleads-connect && npm run dev
```

### Production (Docker)
```bash
docker-compose up -d
```

**Both modes work!** Docker is not required for development.

---

## Resource Requirements:

### Minimum:
- CPU: 2 cores
- RAM: 2GB
- Disk: 5GB

### Recommended:
- CPU: 4 cores
- RAM: 4GB
- Disk: 10GB

### Heavy Usage (100+ concurrent jobs):
- CPU: 8 cores
- RAM: 8GB
- Disk: 20GB

---

## Security Checklist:

Before deploying to production:

- [ ] Change JWT_SECRET from default
- [ ] Set strong database passwords if using external DB
- [ ] Enable HTTPS (use reverse proxy like nginx)
- [ ] Restrict CORS if needed
- [ ] Set up firewall rules
- [ ] Regular backups of ./data directory
- [ ] Monitor logs for suspicious activity

---

## Backup & Restore:

### Backup
```bash
# Backup database and results
tar -czf propleads-backup-$(date +%Y%m%d).tar.gz ./data
```

### Restore
```bash
# Stop containers
docker-compose down

# Restore data
tar -xzf propleads-backup-YYYYMMDD.tar.gz

# Start containers
docker-compose up -d
```

---

## Updates:

### Update to Latest Code
```bash
# Pull latest code
git pull

# Rebuild and restart
docker-compose down
docker-compose up --build -d
```

---

## Summary:

**To deploy PropLeads:**
1. Clone repository
2. Run `docker-compose up -d`
3. Done! ✅

**No installation of:**
- ❌ Go
- ❌ Python
- ❌ Node.js
- ❌ Chrome
- ❌ xvfb
- ❌ Dependencies

**Everything is containerized!** 🐳
