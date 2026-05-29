package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/store"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler exposes register and login.
type AuthHandler struct {
	db  *store.DB
	svc *auth.Service
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(db *store.DB, svc *auth.Service) *AuthHandler {
	return &AuthHandler{db: db, svc: svc}
}

// RegisterRoutes mounts /auth/register and /auth/login on r (the public group).
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/auth")
	g.GET("/register", h.RegisterPage)
	g.POST("/register", h.Register)
	g.GET("/login", h.LoginPage)
	g.POST("/login", h.Login)
	g.GET("/home", h.HomePage)
}

const registerHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Register - QuantSaaS</title>
<style>body{font-family:sans-serif;max-width:400px;margin:80px auto;padding:0 16px}
input{display:block;width:100%;margin:8px 0;padding:8px;box-sizing:border-box}
button{padding:10px 24px;background:#1a73e8;color:#fff;border:none;cursor:pointer;width:100%}
#result{margin-top:12px;padding:10px;background:#f5f5f5;white-space:pre-wrap;display:none}</style></head>
<body><h2>Register</h2>
<input id="email" type="email" placeholder="Email">
<input id="pass" type="password" placeholder="Password (min 8 chars)">
<button onclick="doRegister()">Register</button>
<div id="result"></div>
<p><a href="/api/v1/auth/login">Already have an account? Login</a></p>
<script>
async function doRegister(){
  const r=await fetch('/api/v1/auth/register',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({email:document.getElementById('email').value,password:document.getElementById('pass').value})});
  const data=await r.json();
  if(r.ok&&data.token){
    localStorage.setItem('qs_token',data.token);
    localStorage.setItem('qs_email',data.email||'');
    window.location.href='/api/v1/auth/home';
    return;
  }
  const d=document.getElementById('result');d.style.display='block';
  d.textContent=JSON.stringify(data,null,2);
}
</script></body></html>`

const loginHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Login - QuantSaaS</title>
<style>body{font-family:sans-serif;max-width:400px;margin:80px auto;padding:0 16px}
input{display:block;width:100%;margin:8px 0;padding:8px;box-sizing:border-box}
button{padding:10px 24px;background:#1a73e8;color:#fff;border:none;cursor:pointer;width:100%}
#result{margin-top:12px;padding:10px;background:#f5f5f5;white-space:pre-wrap;display:none}</style></head>
<body><h2>Login</h2>
<input id="email" type="email" placeholder="Email">
<input id="pass" type="password" placeholder="Password">
<button onclick="doLogin()">Login</button>
<div id="result"></div>
<p><a href="/api/v1/auth/register">No account? Register</a></p>
<script>
async function doLogin(){
  const r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({email:document.getElementById('email').value,password:document.getElementById('pass').value})});
  const data=await r.json();
  if(r.ok&&data.token){
    localStorage.setItem('qs_token',data.token);
    localStorage.setItem('qs_email',data.email||'');
    window.location.href='/api/v1/auth/home';
    return;
  }
  const d=document.getElementById('result');d.style.display='block';
  d.textContent=JSON.stringify(data,null,2);
}
</script></body></html>`

func (h *AuthHandler) RegisterPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(registerHTML))
}

func (h *AuthHandler) LoginPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(loginHTML))
}

// homeHTML is the post-login landing view. It is a static shell: auth is done
// client-side using the JWT saved in localStorage at login time, and the page
// pulls live data from the JWT-protected /api/v1/dashboard endpoint. A missing
// or rejected token bounces the visitor back to the login page.
const homeHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Home - QuantSaaS</title>
<style>body{font-family:sans-serif;max-width:760px;margin:40px auto;padding:0 16px}
header{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #eee;padding-bottom:12px}
button{padding:8px 18px;background:#1a73e8;color:#fff;border:none;cursor:pointer;border-radius:4px}
.cards{display:flex;gap:12px;margin:20px 0;flex-wrap:wrap}
.card{flex:1;min-width:140px;background:#f5f7fa;border-radius:6px;padding:16px}
.card .n{font-size:24px;font-weight:600}
.card .l{color:#666;font-size:13px}
table{width:100%;border-collapse:collapse;margin-top:8px}
th,td{text-align:left;padding:8px;border-bottom:1px solid #eee;font-size:14px}
#err{color:#c00;margin-top:16px}</style></head>
<body>
<header><h2>QuantSaaS</h2><div><span id="who"></span> <button onclick="logout()">Logout</button></div></header>
<div class="cards">
  <div class="card"><div class="n" id="ti">-</div><div class="l">Instances</div></div>
  <div class="card"><div class="n" id="rc">-</div><div class="l">Running</div></div>
  <div class="card"><div class="n" id="tc">-</div><div class="l">Total CNY</div></div>
  <div class="card"><div class="n" id="te">-</div><div class="l">Total Equity</div></div>
</div>
<h3>Strategy Instances</h3>
<table><thead><tr><th>ID</th><th>Template</th><th>Status</th><th>CNY</th><th>Equity</th></tr></thead>
<tbody id="rows"><tr><td colspan="5">Loading…</td></tr></tbody></table>
<div id="err"></div>
<script>
const token=localStorage.getItem('qs_token');
if(!token){window.location.replace('/api/v1/auth/login');}
document.getElementById('who').textContent=localStorage.getItem('qs_email')||'';
function logout(){localStorage.removeItem('qs_token');localStorage.removeItem('qs_email');window.location.replace('/api/v1/auth/login');}
async function load(){
  const r=await fetch('/api/v1/dashboard',{headers:{'Authorization':'Bearer '+token}});
  if(r.status===401){logout();return;}
  if(!r.ok){document.getElementById('err').textContent='Error loading dashboard: HTTP '+r.status;return;}
  const d=await r.json();
  document.getElementById('ti').textContent=d.total_instances;
  document.getElementById('rc').textContent=d.running_count;
  document.getElementById('tc').textContent=(d.total_cny||0).toFixed(2);
  document.getElementById('te').textContent=(d.total_equity||0).toFixed(2);
  const rows=document.getElementById('rows');
  if(!d.instances||d.instances.length===0){rows.innerHTML='<tr><td colspan="5">No instances yet.</td></tr>';return;}
  rows.innerHTML=d.instances.map(i=>'<tr><td>'+i.id+'</td><td>'+i.template_id+'</td><td>'+i.status+'</td><td>'+(i.cny_balance||0).toFixed(2)+'</td><td>'+(i.total_equity||0).toFixed(2)+'</td></tr>').join('');
}
if(token){load();}
</script></body></html>`

func (h *AuthHandler) HomePage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(homeHTML))
}

type authRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register godoc
// POST /api/v1/auth/register
// Creates a new user with the default "free" plan and "user" role,
// then returns a freshly minted JWT for convenience.
func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var existing store.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	u := store.User{Email: req.Email, PasswordHash: string(hash), Plan: "free"}
	if err := h.db.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create user: " + err.Error()})
		return
	}

	token, err := h.svc.SignToken(u.ID, "user")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"user_id": u.ID,
		"email":   u.Email,
		"plan":    u.Plan,
		"token":   token,
	})
}

// Login godoc
// POST /api/v1/auth/login
// Verifies credentials and returns a JWT. The token's `role` claim is "user"
// for normal users; lab/dev role assignment is out of scope for this endpoint
// and is administered separately (e.g. via a Plan-style column or admin tool).
func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var u store.User
	if err := h.db.Where("email = ?", req.Email).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	role := planToRole(u.Plan)
	token, err := h.svc.SignToken(u.ID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": u.ID,
		"email":   u.Email,
		"plan":    u.Plan,
		"role":    role,
		"token":   token,
	})
}

// planToRole maps a subscription plan to the JWT role claim.
// "elite" plan members receive lab access; everyone else is a regular user.
// A dedicated admin tool is the right way to elevate to dev — keep that out
// of the user-facing login path.
func planToRole(plan string) string {
	if plan == "elite" {
		return "lab"
	}
	return "user"
}
