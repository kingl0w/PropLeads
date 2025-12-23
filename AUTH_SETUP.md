# 🔐 PropLeads Authentication System

## ✅ What's Been Implemented:

### Backend (Go):
1. **SQLite Database** (`data/propleads.db`)
   - User table with email, username, password hash
   - Subscription status/tier fields (ready for payments)
   - Auto-created on server start

2. **Password Security**
   - bcrypt hashing (industry standard)
   - Minimum 8 characters required
   - Salted hashes stored

3. **JWT Tokens**
   - 24-hour expiration
   - Signed with secret key
   - Contains user info + subscription status

4. **API Endpoints**:
   - `POST /api/auth/signup` - Create account
   - `POST /api/auth/login` - Login
   - `POST /api/scrape` - **PROTECTED** (requires auth)
   - `GET /api/scrape/{jobId}/status` - **PROTECTED**

5. **Security Features**:
   - Unique email/username validation
   - SQL injection prevention
   - Password not returned in responses
   - Account active/inactive status

### Frontend (React/TypeScript):
1. **Login Page** (`/login`)
2. **Signup Page** (`/signup`)
3. **Auth Context** - Manages authentication state
4. **Token Storage** - localStorage (persists across sessions)
5. **Protected Routes** - Auto-redirects if not logged in
6. **Auth Headers** - JWT sent with every API request

---

## 🚀 How To Use:

### First Time Setup:

1. **Start the backend:**
```bash
./server
```
Database will be created automatically at `data/propleads.db`

2. **Start the frontend:**
```bash
cd propleads-connect
npm run dev
```

3. **Create an account:**
   - Go to `http://localhost:5173/signup`
   - Enter email, username, password (min 8 chars)
   - Click "Sign up"
   - Automatically logged in

4. **Use the app:**
   - Upload PIDs and scrape data
   - Your session persists (24 hours)

---

## 🔒 Security Features:

### What's Protected:
- ✅ All scraping endpoints require authentication
- ✅ Passwords hashed with bcrypt
- ✅ JWT tokens expire after 24 hours
- ✅ Email/username uniqueness enforced
- ✅ SQL injection prevented
- ✅ CORS configured properly

### Ready for Future Payments:
- `subscription_status` field: free, active, expired
- `subscription_tier` field: basic, pro, enterprise
- Easy to add middleware check:
  ```go
  if claims.SubscriptionStatus != "active" {
      return "Premium feature - subscription required"
  }
  ```

---

## 💳 Adding Payment Integration Later:

When you're ready to add payments (Stripe, PayPal, etc):

1. **Add payment endpoint:**
```go
router.HandleFunc("/api/payment/subscribe", handleSubscribe).Methods("POST")
```

2. **Update subscription on payment:**
```go
auth.UpdateSubscriptionStatus(userID, "active", "pro")
```

3. **Protect premium features:**
```go
// In jwt.go AuthMiddleware, uncomment:
if claims.SubscriptionStatus != "active" {
    http.Error(w, "Active subscription required", http.StatusForbidden)
    return
}
```

4. **Add webhook for payment events:**
```go
router.HandleFunc("/api/webhooks/stripe", handleStripeWebhook).Methods("POST")
```

---

## 📝 Current Users Table Schema:

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT UNIQUE NOT NULL,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  subscription_status TEXT DEFAULT 'free',
  subscription_tier TEXT DEFAULT 'basic',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_login DATETIME,
  is_active BOOLEAN DEFAULT 1
);
```

---

## ⚙️ Configuration:

### JWT Secret (IMPORTANT for production):
Set environment variable before starting server:
```bash
export JWT_SECRET="your-super-secret-key-here"
./server
```

If not set, uses default (change this in production!)

### Database Location:
`data/propleads.db` (created automatically)

### Token Expiration:
24 hours (configurable in `internal/auth/jwt.go`)

---

## 🧪 Testing:

### Test Signup:
```bash
curl -X POST http://localhost:8080/api/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","username":"testuser","password":"password123"}'
```

### Test Login:
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

### Test Protected Endpoint:
```bash
curl -X POST http://localhost:8080/api/scrape \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{"county":"newhanover","pids":["123","456"]}'
```

---

## 🎯 Summary:

**Before:** Anyone could use the API
**After:** Must create account and login to use

**Security Level:**
- ✅ Production-ready authentication
- ✅ Industry-standard password hashing
- ✅ Secure token-based auth
- ✅ Future-proof for payments

**No one can access your scraping service without an account!** 🔒
