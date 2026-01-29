package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const dataPath = "data/lottery.json"

type LotteryEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Ticket    string `json:"ticket"`
	CreatedAt string `json:"created_at"`
}

type AddEntryRequest struct {
	Name string `json:"name"`
}


var (
	entries   []LotteryEntry
	entriesMu sync.RWMutex
	nextID    int
)

func main() {
	rand.Seed(time.Now().UnixNano())
	if err := loadEntries(); err != nil {
		entries = make([]LotteryEntry, 0)
	}
	r := gin.Default()

	r.GET("/entries", listEntries)
	r.POST("/entries", addEntry)
	r.DELETE("/entries/:id", deleteEntry)
	r.GET("/draw", drawLottery)
	r.GET("/openapi.yaml", serveOpenAPI)
	r.GET("/swagger", serveSwaggerUI)
	r.GET("/", serveLanding)

	host := "0.0.0.0"
	if h := os.Getenv("HOST"); h != "" {
		host = h
	}
	port := "9191"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	addr := host + ":" + port
	r.Run(addr)
}

func loadEntries() error {
	entriesMu.Lock()
	defer entriesMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		return err
	}
	b, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			entries = make([]LotteryEntry, 0)
			nextID = 1
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &entries); err != nil {
		return err
	}
	nextID = 0
	for _, e := range entries {
		if id, err := strconv.Atoi(e.ID); err == nil && id >= nextID {
			nextID = id + 1
		}
	}
	if nextID == 0 {
		nextID = 1
	}
	return nil
}

func saveEntries() error {
	return saveEntriesSnapshot(entries)
}

// saveEntriesSnapshot writes a copy of entries to disk (call without holding entriesMu).
func saveEntriesSnapshot(snapshot []LotteryEntry) error {
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataPath, b, 0644)
}

func listEntries(c *gin.Context) {
	entriesMu.RLock()
	defer entriesMu.RUnlock()
	c.JSON(http.StatusOK, entries)
}

func ticketExists(ticket string) bool {
	for _, e := range entries {
		if e.Ticket == ticket {
			return true
		}
	}
	return false
}

func generateTicketUnderLock() string {
	for i := 0; i < 20; i++ {
		t := "TKT-" + fmt.Sprintf("%06d", rand.Intn(1000000))
		if !ticketExists(t) {
			return t
		}
	}
	return "TKT-" + fmt.Sprintf("%06d", rand.Intn(1000000))
}

func addEntry(c *gin.Context) {
	var req AddEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: name required"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	entriesMu.Lock()
	for _, e := range entries {
		if strings.EqualFold(e.Name, name) {
			entriesMu.Unlock()
			c.JSON(http.StatusConflict, gin.H{"error": "name already registered"})
			return
		}
	}
	nextID++
	id := fmt.Sprintf("%d", nextID)
	ticket := generateTicketUnderLock()
	entry := LotteryEntry{
		ID:        id,
		Name:      name,
		Ticket:    ticket,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	entries = append(entries, entry)
	snapshot := make([]LotteryEntry, len(entries))
	copy(snapshot, entries)
	entriesMu.Unlock()
	c.JSON(http.StatusCreated, entry)
	go func() { _ = saveEntriesSnapshot(snapshot) }()
}

func deleteEntry(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry id required"})
		return
	}
	entriesMu.Lock()
	var kept []LotteryEntry
	for _, e := range entries {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(entries) {
		entriesMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	entries = kept
	snapshot := make([]LotteryEntry, len(entries))
	copy(snapshot, entries)
	entriesMu.Unlock()
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.JSON(http.StatusOK, gin.H{"deleted": 1})
	go func() { _ = saveEntriesSnapshot(snapshot) }()
}

func drawLottery(c *gin.Context) {
	entriesMu.Lock()
	if len(entries) == 0 {
		entriesMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "no entries to draw"})
		return
	}
	idx := rand.Intn(len(entries))
	winner := entries[idx]
	entries = make([]LotteryEntry, 0)
	nextID = 0
	entriesMu.Unlock()
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.JSON(http.StatusOK, gin.H{
		"winner":  winner,
		"message": "Winner: " + winner.Name + " – Ticket: " + winner.Ticket,
	})
	go func() { _ = saveEntriesSnapshot([]LotteryEntry{}) }()
}

func serveOpenAPI(c *gin.Context) {
	c.Header("Content-Type", "application/x-yaml")
	c.File("openapi.yaml")
}

func serveLanding(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, landingHTML)
}

func serveSwaggerUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerHTML)
}

const landingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NTNX Lottery</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,600;0,9..40,700;1,9..40,400&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #0f0f12;
      --surface: #1a1a20;
      --surface2: #24242c;
      --text: #e8e8ec;
      --text-muted: #8888a0;
      --accent: #7c5cff;
      --accent-hover: #9378ff;
      --success: #22c55e;
      --success-bg: rgba(34, 197, 94, 0.12);
      --error: #ef4444;
      --error-bg: rgba(239, 68, 68, 0.12);
      --radius: 12px;
      --shadow: 0 4px 24px rgba(0,0,0,0.3);
    }
    * { box-sizing: border-box; }
    body {
      font-family: 'DM Sans', system-ui, sans-serif;
      background: var(--bg);
      color: var(--text);
      max-width: 32rem;
      margin: 0 auto;
      padding: 2rem 1.25rem;
      min-height: 100vh;
      line-height: 1.5;
    }
    h1 {
      font-size: 1.75rem;
      font-weight: 700;
      margin: 0 0 0.25rem 0;
      letter-spacing: -0.02em;
      background: linear-gradient(135deg, var(--text) 0%, var(--text-muted) 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }
    .subtitle { color: var(--text-muted); font-size: 0.9rem; margin-bottom: 1.75rem; }
    section {
      background: var(--surface);
      border: 1px solid var(--surface2);
      border-radius: var(--radius);
      padding: 1.25rem 1.5rem;
      margin-bottom: 1.25rem;
      box-shadow: var(--shadow);
    }
    section h2 {
      font-size: 0.85rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
      margin: 0 0 1rem 0;
    }
    label { display: block; margin-bottom: 0.35rem; font-weight: 600; font-size: 0.9rem; color: var(--text); }
    input[type="text"] {
      width: 100%;
      padding: 0.65rem 1rem;
      font-size: 1rem;
      font-family: inherit;
      background: var(--surface2);
      border: 1px solid transparent;
      border-radius: 8px;
      color: var(--text);
      transition: border-color 0.2s, box-shadow 0.2s;
    }
    input[type="text"]:focus {
      outline: none;
      border-color: var(--accent);
      box-shadow: 0 0 0 3px rgba(124, 92, 255, 0.2);
    }
    input::placeholder { color: var(--text-muted); }
    button, .btn {
      display: inline-block;
      margin-top: 0.75rem;
      padding: 0.65rem 1.35rem;
      background: var(--accent);
      color: #fff;
      border: none;
      border-radius: 8px;
      font-size: 0.95rem;
      font-weight: 600;
      font-family: inherit;
      cursor: pointer;
      text-decoration: none;
      transition: background 0.2s, transform 0.1s;
    }
    button:hover, .btn:hover { background: var(--accent-hover); }
    button:active, .btn:active { transform: scale(0.98); }
    button:disabled { background: var(--surface2); color: var(--text-muted); cursor: not-allowed; transform: none; }
    .btn-secondary { background: var(--surface2); color: var(--text); }
    .btn-secondary:hover { background: var(--surface2); color: var(--accent); }
    .entries-section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.5rem; }
    .entries-section-header h2 { margin: 0; }
    .entries-toolbar { display: flex; align-items: center; gap: 0.5rem; }
    .btn-delete-small { font-size: 0.75rem; padding: 0.35rem 0.65rem; background: rgba(239, 68, 68, 0.2); color: var(--error); border: 1px solid rgba(239, 68, 68, 0.4); border-radius: 6px; cursor: pointer; font-weight: 600; font-family: inherit; }
    .btn-delete-small:hover { background: rgba(239, 68, 68, 0.3); }
    .btn-delete-small:disabled { opacity: 0.5; cursor: not-allowed; }
    .btn-done-small { font-size: 0.75rem; padding: 0.35rem 0.65rem; background: var(--surface2); color: var(--text); border: 1px solid var(--surface2); border-radius: 6px; cursor: pointer; font-weight: 600; font-family: inherit; }
    .btn-done-small:hover { color: var(--accent); }
    .entry-delete-btn { background: none; border: none; color: var(--error); cursor: pointer; padding: 0; line-height: 0; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; vertical-align: middle; height: 1.25rem; width: 1.25rem; }
    .entry-delete-btn:hover { background: var(--error-bg); color: var(--error); }
    .entry-delete-btn .icon-minus-circle { width: 1.25rem; height: 1.25rem; display: block; vertical-align: middle; }
    .col-check, .col-delete { width: 2.25rem; text-align: right; padding-left: 0.25rem; }
    td.col-delete { vertical-align: middle; display: flex; align-items: center; justify-content: flex-end; padding-top: 0; padding-bottom: 0; }
    .msg {
      margin-top: 0.75rem;
      padding: 0.75rem 1rem;
      border-radius: 8px;
      font-size: 0.9rem;
      animation: msgIn 0.25s ease-out;
    }
    .msg.success { background: var(--success-bg); color: var(--success); border: 1px solid rgba(34, 197, 94, 0.25); }
    .msg.error { background: var(--error-bg); color: var(--error); border: 1px solid rgba(239, 68, 68, 0.25); }
    .msg.hiding { opacity: 0; transition: opacity 0.3s ease-out; }
    @keyframes msgIn { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }
    table { width: 100%; border-collapse: collapse; margin-top: 0.5rem; font-size: 0.9rem; }
    th, td { text-align: left; padding: 0.6rem 0.5rem; border-bottom: 1px solid var(--surface2); }
    th { font-weight: 600; color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
    tr:last-child td { border-bottom: none; }
    #entries-list.empty { color: var(--text-muted); font-style: italic; padding: 0.75rem 0; }
    .swagger-link { margin-top: 1.5rem; font-size: 0.85rem; }
    .swagger-link .btn { background: var(--surface2); color: var(--text); }
    .swagger-link .btn:hover { background: var(--surface2); color: var(--accent); }
    .winner-overlay {
      display: none;
      position: fixed;
      inset: 0;
      z-index: 1000;
      background: rgba(0,0,0,0.75);
      align-items: center;
      justify-content: center;
      padding: 1.5rem;
      animation: overlayIn 0.3s ease-out;
    }
    .winner-overlay.show { display: flex; }
    @keyframes overlayIn { from { opacity: 0; } to { opacity: 1; } }
    .winner-banner {
      position: relative;
      background: linear-gradient(145deg, #2d1f5c 0%, #1a0f33 100%);
      border: 2px solid rgba(212, 175, 55, 0.5);
      border-radius: 20px;
      padding: 2.5rem 3rem;
      text-align: center;
      max-width: 28rem;
      box-shadow: 0 0 60px rgba(212, 175, 55, 0.3), 0 20px 60px rgba(0,0,0,0.5);
      animation: blastIn 0.6s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
    }
    .winner-blast {
      position: absolute;
      inset: -30%;
      border-radius: 50%;
      background: radial-gradient(circle, rgba(212, 175, 55, 0.25) 0%, transparent 70%);
      animation: blastExpand 0.8s ease-out forwards;
      pointer-events: none;
    }
    @keyframes blastIn {
      from { opacity: 0; transform: scale(0.3); }
      to { opacity: 1; transform: scale(1); }
    }
    @keyframes blastExpand {
      from { opacity: 1; transform: scale(0.2); }
      to { opacity: 0; transform: scale(1.5); }
    }
    .winner-banner h2 {
      margin: 0 0 0.5rem 0;
      font-size: 0.9rem;
      font-weight: 600;
      letter-spacing: 0.2em;
      color: #d4af37;
      text-transform: uppercase;
    }
    .winner-banner .winner-name {
      font-size: 2rem;
      font-weight: 700;
      color: #fff;
      margin: 0.5rem 0 0.25rem 0;
      letter-spacing: -0.02em;
      line-height: 1.2;
    }
    .winner-banner .winner-ticket {
      font-size: 1.1rem;
      color: #d4af37;
      margin: 0.25rem 0 1rem 0;
      font-weight: 600;
    }
    .winner-banner .winner-dismiss {
      margin-top: 1rem;
      padding: 0.5rem 1.5rem;
      font-size: 0.9rem;
      background: rgba(212, 175, 55, 0.2);
      color: #d4af37;
      border: 1px solid rgba(212, 175, 55, 0.4);
      border-radius: 8px;
      cursor: pointer;
      font-weight: 600;
    }
    .winner-banner .winner-dismiss:hover { background: rgba(212, 175, 55, 0.3); }
  </style>
</head>
<body>
  <h1 id="page-title">NTNX Lottery</h1>
  <p class="subtitle">Add your name, get a ticket. Draw when ready.</p>
  <div id="winner-overlay" class="winner-overlay">
    <div class="winner-banner">
      <div class="winner-blast"></div>
      <h2>Congratulations</h2>
      <div class="winner-name" id="winner-name"></div>
      <div class="winner-ticket" id="winner-ticket"></div>
      <button type="button" class="winner-dismiss" id="winner-dismiss">Close</button>
    </div>
  </div>
  <section>
    <h2>Add entry</h2>
    <label for="name">Name (unique)</label>
    <input type="text" id="name" placeholder="Your name" />
    <button type="button" id="add-btn">Add entry</button>
    <div id="add-msg" class="msg" style="display:none;"></div>
  </section>
  <section>
    <div class="entries-section-header">
      <h2>Entries</h2>
      <div class="entries-toolbar">
        <button type="button" id="delete-entries-btn" class="btn-delete-small">Delete entries</button>
        <span id="entries-actions" style="display:none;">
          <button type="button" id="done-delete-btn" class="btn-done-small">Done</button>
        </span>
      </div>
    </div>
    <div id="entries-list">Loading…</div>
  </section>
  <section>
    <h2>Draw</h2>
    <button type="button" id="draw-btn">Draw lottery</button>
    <div id="draw-msg" class="msg" style="display:none;"></div>
  </section>
  <p class="swagger-link"><a href="/swagger" class="btn">Open Swagger UI</a></p>
  <script>
    var addMsg = document.getElementById('add-msg');
    var addBtn = document.getElementById('add-btn');
    var nameInput = document.getElementById('name');
    var entriesList = document.getElementById('entries-list');
    var drawBtn = document.getElementById('draw-btn');
    var drawMsg = document.getElementById('draw-msg');
    var addMsgTimer = null;
    var drawMsgTimer = null;
    var addHideTimer = null;
    var drawHideTimer = null;
    var winnerOverlayTimer = null;
    var MSG_AUTO_HIDE_MS = 5000;
    var WINNER_BANNER_MS = 8000;
    var winnerOverlay = document.getElementById('winner-overlay');
    var winnerNameEl = document.getElementById('winner-name');
    var winnerTicketEl = document.getElementById('winner-ticket');
    var winnerDismiss = document.getElementById('winner-dismiss');
    var currentEntries = [];
    var deleteEntriesBtn = document.getElementById('delete-entries-btn');
    var entriesActions = document.getElementById('entries-actions');
    var doneDeleteBtn = document.getElementById('done-delete-btn');

    function hideWinnerBanner() {
      if (winnerOverlayTimer) { clearTimeout(winnerOverlayTimer); winnerOverlayTimer = null; }
      if (winnerOverlay) winnerOverlay.classList.remove('show');
    }
    function showWinnerBanner(name, ticket) {
      if (winnerOverlayTimer) clearTimeout(winnerOverlayTimer);
      if (winnerNameEl) winnerNameEl.textContent = name;
      if (winnerTicketEl) winnerTicketEl.textContent = 'Ticket: ' + ticket;
      if (winnerOverlay) winnerOverlay.classList.add('show');
      winnerOverlayTimer = setTimeout(hideWinnerBanner, WINNER_BANNER_MS);
    }
    if (winnerDismiss) winnerDismiss.addEventListener('click', hideWinnerBanner);

    function hideAddMsg() {
      if (addMsgTimer) { clearTimeout(addMsgTimer); addMsgTimer = null; }
      addMsg.classList.add('hiding');
      if (addHideTimer) clearTimeout(addHideTimer);
      addHideTimer = setTimeout(function() {
        addMsg.style.display = 'none';
        addMsg.classList.remove('hiding');
        addHideTimer = null;
      }, 300);
    }
    function hideDrawMsg() {
      if (drawMsgTimer) { clearTimeout(drawMsgTimer); drawMsgTimer = null; }
      drawMsg.classList.add('hiding');
      if (drawHideTimer) clearTimeout(drawHideTimer);
      drawHideTimer = setTimeout(function() {
        drawMsg.style.display = 'none';
        drawMsg.classList.remove('hiding');
        drawHideTimer = null;
      }, 300);
    }
    function showAddMsg(text, isError) {
      if (addMsgTimer) clearTimeout(addMsgTimer);
      if (addHideTimer) { clearTimeout(addHideTimer); addHideTimer = null; }
      addMsg.textContent = text;
      addMsg.className = 'msg ' + (isError ? 'error' : 'success');
      addMsg.style.display = 'block';
      addMsg.classList.remove('hiding');
      addMsgTimer = setTimeout(hideAddMsg, MSG_AUTO_HIDE_MS);
    }
    function showDrawMsg(text, isError) {
      if (drawMsgTimer) clearTimeout(drawMsgTimer);
      if (drawHideTimer) { clearTimeout(drawHideTimer); drawHideTimer = null; }
      drawMsg.textContent = text;
      drawMsg.className = 'msg ' + (isError ? 'error' : 'success');
      drawMsg.style.display = 'block';
      drawMsg.classList.remove('hiding');
      drawMsgTimer = setTimeout(hideDrawMsg, MSG_AUTO_HIDE_MS);
    }

    function renderEntries(arr, deleteMode) {
      if (arr.length === 0) {
        entriesList.innerHTML = '<span class="empty">No entries yet.</span>';
        entriesList.className = 'empty';
        return;
      }
      entriesList.className = '';
      var table = '<table><thead><tr><th>Name</th><th>Ticket</th>';
      if (deleteMode) table += '<th class="col-delete"></th>';
      table += '</tr></thead><tbody>';
      arr.forEach(function(e) {
        table += '<tr><td>' + escapeHtml(e.name) + '</td><td>' + escapeHtml(e.ticket) + '</td>';
        if (deleteMode) table += '<td class="col-delete"><button type="button" class="entry-delete-btn" data-id="' + escapeHtml(e.id) + '" title="Delete entry"><svg class="icon-minus-circle" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="8" y1="12" x2="16" y2="12"/></svg></button></td>';
        table += '</tr>';
      });
      table += '</tbody></table>';
      entriesList.innerHTML = table;
    }
    function loadEntries() {
      entriesActions.style.display = 'none';
      fetch('/entries').then(function(r) { return r.json(); }).then(function(arr) {
        currentEntries = arr || [];
        renderEntries(currentEntries, false);
        if (deleteEntriesBtn) deleteEntriesBtn.disabled = currentEntries.length === 0;
      }).catch(function() {
        entriesList.innerHTML = '<span class="error">Failed to load entries.</span>';
        if (deleteEntriesBtn) deleteEntriesBtn.disabled = true;
      });
    }
    function escapeHtml(s) {
      var div = document.createElement('div');
      div.textContent = s;
      return div.innerHTML;
    }

    addBtn.addEventListener('click', function() {
      var name = nameInput.value.trim();
      if (!name) { showAddMsg('Enter a name.', true); return; }
      addBtn.disabled = true;
      fetch('/entries', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name })
      }).then(function(r) {
        return r.json().then(function(data) {
          if (r.status === 201) {
            showAddMsg('Your ticket: ' + data.ticket, false);
            nameInput.value = '';
            loadEntries();
          } else if (r.status === 409) {
            showAddMsg('Name already registered.', true);
          } else {
            showAddMsg(data.error || 'Error', true);
          }
        });
      }).catch(function() { showAddMsg('Request failed.', true); }).finally(function() { addBtn.disabled = false; });
    });

    drawBtn.addEventListener('click', function() {
      drawMsg.style.display = 'none';
      drawMsg.textContent = '';
      drawBtn.disabled = true;
      fetch('/draw').then(function(r) {
        return r.text().then(function(text) {
          var data = null;
          try { data = text ? JSON.parse(text) : null; } catch (e) { data = null; }
          if (!r.ok) {
            showDrawMsg(data && data.error ? data.error : 'No entries to draw.', true);
            return;
          }
          if (!data) {
            showDrawMsg('Invalid response from server.', true);
            return;
          }
          var w = data.winner;
          if (w && typeof w === 'object') {
            showWinnerBanner(String(w.name || ''), String(w.ticket || ''));
            loadEntries();
          } else {
            loadEntries();
            showDrawMsg('Draw completed.', false);
          }
        });
      }).catch(function(err) {
        showDrawMsg('Request failed: ' + (err && err.message ? err.message : 'network error'), true);
      }).finally(function() { drawBtn.disabled = false; });
    });

    if (deleteEntriesBtn) deleteEntriesBtn.addEventListener('click', function() {
      if (currentEntries.length === 0) return;
      renderEntries(currentEntries, true);
      entriesActions.style.display = 'inline-flex';
    });
    if (doneDeleteBtn) doneDeleteBtn.addEventListener('click', function() { loadEntries(); });
    entriesList.addEventListener('click', function(ev) {
      var btn = ev.target.closest('.entry-delete-btn');
      if (!btn) return;
      var id = btn.getAttribute('data-id');
      if (!id) return;
      btn.disabled = true;
      fetch('/entries/' + encodeURIComponent(id), { method: 'DELETE' }).then(function(r) {
        if (r.ok) {
          fetch('/entries').then(function(res) { return res.json(); }).then(function(arr) {
            currentEntries = arr || [];
            renderEntries(currentEntries, true);
            if (deleteEntriesBtn) deleteEntriesBtn.disabled = currentEntries.length === 0;
            if (currentEntries.length === 0) entriesActions.style.display = 'none';
          });
        }
      }).catch(function() { loadEntries(); }).finally(function() { btn.disabled = false; });
    });

    loadEntries();
  </script>
</body>
</html>
`

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Lottery API - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ]
      });
    };
  </script>
</body>
</html>
`
