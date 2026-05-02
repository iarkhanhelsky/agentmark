/* global Alpine, marked */

function shell() {
  return {
    markdown: "",
    threads: [],
    renderedHtml: "",
    fileMeta: { name: "", path: "" },
    viewMode: "preview",
    showResolved: false,
    historyOpen: false,
    activeThreadId: null,
    gutterLayout: [],
    bubble: { open: false, top: 0 },
    drafts: {},
    toolbar: { show: false, top: 0, left: 0 },
    sel: { text: "", prefix: "", suffix: "" },
    snapshots: [],
    diffLines: [],
    history: {
      left: "",
      right: "",
      focusIndex: 0,
      hunks: [],
      accept: {},
    },
    ws: null,
    _scrollScheduled: false,
    _pendingOpenId: null,

    get visibleThreads() {
      return this.threads.filter((t) => !t.detached);
    },

    get commentCount() {
      return this.visibleThreads.length;
    },

    get canNavigateComments() {
      return this.viewMode === "preview" && this.unresolvedNavList().length > 0;
    },

    get activeThread() {
      return this.visibleThreads.find((t) => t.id === this.activeThreadId) || null;
    },

    get diffRangeLabel() {
      const a = this.history.left;
      const b = this.history.right;
      if (!a || !b) return "";
      return `${shortId(a)} → ${shortId(b)}`;
    },

    init() {
      this.fetchFileMeta();
      this.connectWS();
    },

    async fetchFileMeta() {
      try {
        const res = await fetch("/api/file-meta");
        if (res.ok) {
          this.fileMeta = await res.json();
        }
      } catch (_) {
        /* ignore */
      }
    },

    connectWS() {
      const proto = window.location.protocol === "https:" ? "wss" : "ws";
      this.ws = new WebSocket(`${proto}://${window.location.host}/ws`);
      this.ws.onmessage = (ev) => {
        const msg = JSON.parse(ev.data);
        if (msg.type === "file_update") {
          this.markdown = msg.content;
          this.render();
        } else if (msg.type === "threads_update") {
          this.threads = msg.threads || [];
          if (this._pendingOpenId) {
            const pid = this._pendingOpenId;
            this._pendingOpenId = null;
            this.$nextTick(() => this.openThread(pid));
          }
          this.render();
        } else if (msg.type === "snapshot_saved") {
          if (this.historyOpen) this.loadSnapshots();
        }
      };
    },

    formatSnapLabel(snap, idx) {
      const n = this.snapshots.length;
      const v = n - idx;
      return `v${v} · ${shortId(snap.id)}`;
    },

    bubbleAuthorLabel(thread) {
      if (!thread) return "Comment";
      const msgs = thread.thread || [];
      if (!msgs.length) return "New note";
      return "You";
    },

    render() {
      const raw = this.markdown || "";
      if (this.viewMode === "raw") {
        this.renderedHtml = "";
        this.$nextTick(() => {
          this.gutterLayout = [];
        });
        return;
      }
      let html = marked.parse(raw, { mangle: false, headerIds: false });
      html = highlightAnchors(html, this.visibleThreads, this.showResolved);
      this.renderedHtml = html;
      this.$nextTick(() => {
        this.wireMarks();
        this.updateGutterPins();
      });
    },

    wireMarks() {
      document.querySelectorAll(".anchor-mark").forEach((el) => {
        el.onclick = (e) => {
          e.preventDefault();
          e.stopPropagation();
          const id = el.getAttribute("data-thread-id");
          this.openThread(id, e);
        };
      });
    },

    updateGutterPins() {
      if (this.viewMode !== "preview") {
        this.gutterLayout = [];
        return;
      }
      const root = document.querySelector(".doc-review-row");
      if (!root) {
        this.gutterLayout = [];
        return;
      }
      const seen = new Set();
      const pins = [];
      document.querySelectorAll(".anchor-mark[data-thread-id]").forEach((el) => {
        const id = el.getAttribute("data-thread-id");
        if (!id || seen.has(id)) return;
        seen.add(id);
        const t = this.visibleThreads.find((x) => x.id === id);
        if (!t) return;
        if (t.resolved && !this.showResolved) return;
        const er = el.getBoundingClientRect();
        const rr = root.getBoundingClientRect();
        let y = er.top - rr.top + er.height / 2;
        pins.push({ id, y, resolved: !!t.resolved });
      });
      pins.sort((a, b) => a.y - b.y);
      const minGap = 32;
      for (let i = 1; i < pins.length; i++) {
        if (pins[i].y < pins[i - 1].y + minGap) {
          pins[i].y = pins[i - 1].y + minGap;
        }
      }
      this.gutterLayout = pins;
    },

    onDocScroll() {
      if (this._scrollScheduled) return;
      this._scrollScheduled = true;
      requestAnimationFrame(() => {
        this._scrollScheduled = false;
        this.updateGutterPins();
        if (this.activeThreadId && this.bubble.open) {
          this.positionBubbleForThread(this.activeThreadId, null);
        }
      });
    },

    openThread(id, ev) {
      this.toolbar.show = false;
      this.activeThreadId = id;
      this.bubble.open = true;
      const evRef = ev;
      this.$nextTick(() => this.positionBubbleForThread(id, evRef));
    },

    positionBubbleForThread(id, ev) {
      const rail = document.getElementById("bubble-rail");
      if (!rail) return;
      const mark = document.querySelector(`.anchor-mark[data-thread-id="${id}"]`);
      if (mark) {
        const mr = mark.getBoundingClientRect();
        const rr = rail.getBoundingClientRect();
        let top = mr.top - rr.top + mr.height / 2 - 24;
        this.bubble = { open: true, top: Math.max(8, top) };
        return;
      }
      if (ev && ev.currentTarget && ev.currentTarget.getBoundingClientRect) {
        const pr = ev.currentTarget.getBoundingClientRect();
        const rr = rail.getBoundingClientRect();
        let top = pr.top - rr.top + pr.height / 2 - 24;
        this.bubble = { open: true, top: Math.max(8, top) };
      }
    },

    closeThread() {
      this.activeThreadId = null;
      this.bubble.open = false;
    },

    onPreviewMouseUp() {
      const selObj = window.getSelection();
      if (!selObj || selObj.rangeCount === 0) {
        this.toolbar.show = false;
        return;
      }
      const text = selObj.toString().trim();
      if (!text) {
        this.toolbar.show = false;
        return;
      }
      const range = selObj.getRangeAt(0);
      const preview = document.getElementById("preview");
      if (!preview || !preview.contains(range.commonAncestorContainer)) {
        this.toolbar.show = false;
        return;
      }
      const row = document.getElementById("doc-review-row");
      if (!row) return;
      const rect = range.getBoundingClientRect();
      const rr = row.getBoundingClientRect();
      this.toolbar = {
        show: true,
        top: rect.top - rr.top - 42,
        left: rect.left - rr.left + rect.width / 2,
      };
      const full = this.markdown;
      const ctx = contextAroundSelection(full, text);
      this.sel = { text, prefix: ctx.prefix, suffix: ctx.suffix };
    },

    async addCommentFromSelection() {
      const id = crypto.randomUUID();
      const hint = this.markdown.indexOf(this.sel.text);
      const body = {
        id,
        anchorText: this.sel.text,
        anchor:
          hint >= 0
            ? {
                startOffset: hint,
                endOffset: hint + this.sel.text.length,
                prefix: this.sel.prefix,
                suffix: this.sel.suffix,
              }
            : undefined,
      };
      this._pendingOpenId = id;
      await fetch("/api/threads/upsert", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      this.toolbar.show = false;
      window.getSelection()?.removeAllRanges();
    },

    async sendReply(threadId) {
      const body = (this.drafts[threadId] || "").trim();
      if (!body) return;
      await fetch("/api/threads/reply", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: threadId, body }),
      });
      this.drafts[threadId] = "";
    },

    async toggleResolve(threadId, resolved) {
      await fetch("/api/threads/resolve", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: threadId, resolved }),
      });
    },

    copyReviewContext() {
      const path = this.fileMeta.name || "document";
      let out = `Review context for: ${path}\n\n`;
      let i = 1;
      for (const t of this.visibleThreads) {
        if (t.resolved && !this.showResolved) continue;
        out += `[Thread ${i}] Anchor: ${JSON.stringify(t.anchorText)}\n`;
        for (const m of t.thread || []) {
          out += `> ${m.role}: ${m.body}\n`;
        }
        const d = (this.drafts[t.id] || "").trim();
        if (d) out += `> (draft): ${d}\n`;
        out += "\n";
        i++;
      }
      out += "\n(Paste into your IDE agent chat.)\n";
      navigator.clipboard.writeText(out);
    },

    unresolvedNavList() {
      return this.visibleThreads
        .filter((t) => !t.resolved && (!t.detached))
        .sort((a, b) => (a.anchor?.startOffset ?? 0) - (b.anchor?.startOffset ?? 0));
    },

    nextThread() {
      const list = this.unresolvedNavList();
      if (!list.length) return;
      let i = list.findIndex((t) => t.id === this.activeThreadId);
      if (i < 0) i = -1;
      const next = list[(i + 1) % list.length];
      this.openThread(next.id);
      document.querySelector(`.anchor-mark[data-thread-id="${next.id}"]`)?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    },

    prevThread() {
      const list = this.unresolvedNavList();
      if (!list.length) return;
      let i = list.findIndex((t) => t.id === this.activeThreadId);
      if (i < 0) i = 0;
      const prev = list[(i - 1 + list.length) % list.length];
      this.openThread(prev.id);
      document.querySelector(`.anchor-mark[data-thread-id="${prev.id}"]`)?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    },

    async openHistory() {
      this.historyOpen = true;
      this.closeThread();
      await this.loadSnapshots();
      if (this.snapshots.length >= 2) {
        this.selectSnapshotPair(0);
      } else {
        this.diffLines = [];
        this.history.hunks = [];
      }
    },

    async loadSnapshots() {
      const res = await fetch("/api/snapshots");
      this.snapshots = await res.json();
    },

    selectSnapshotPair(idx) {
      if (idx < 0 || idx >= this.snapshots.length) return;
      this.history.focusIndex = idx;
      if (idx + 1 < this.snapshots.length) {
        this.history.left = this.snapshots[idx + 1].id;
        this.history.right = this.snapshots[idx].id;
      } else {
        this.history.left = this.snapshots[idx].id;
        this.history.right = this.snapshots[idx].id;
      }
      this.loadDiff();
    },

    async loadDiff() {
      const a = this.history.left;
      const b = this.history.right;
      if (!a || !b || a === b) {
        this.history.hunks = [];
        this.diffLines = [];
        this.history.accept = {};
        return;
      }
      const res = await fetch(`/api/snapshots/diff?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`);
      const data = await res.json();
      this.history.hunks = data.hunks || [];
      this.history.accept = {};
      for (const h of this.history.hunks) {
        this.history.accept[h.id] = true;
      }
      this.diffLines = hunksToDiffLines(this.history.hunks);
    },

    async applySelectedHunks() {
      const ids = Object.entries(this.history.accept)
        .filter(([, v]) => v)
        .map(([k]) => k);
      const res = await fetch("/api/snapshots/apply", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          leftId: this.history.left,
          rightId: this.history.right,
          acceptedHunkIds: ids,
        }),
      });
      if (!res.ok) {
        alert(await res.text());
        return;
      }
      this.history.hunks = [];
      this.diffLines = [];
      await this.loadSnapshots();
      if (this.snapshots.length >= 2) this.selectSnapshotPair(0);
    },
  };
}

document.addEventListener("alpine:init", () => {
  Alpine.data("shell", shell);
});

function shortId(id) {
  if (!id) return "";
  return id.length > 22 ? id.slice(0, 19) + "…" : id;
}

function hunksToDiffLines(hunks) {
  const lines = [];
  for (const h of hunks) {
    for (const line of h.old || []) {
      lines.push({ type: "removed", text: line });
    }
    for (const line of h.new || []) {
      lines.push({ type: "added", text: line });
    }
  }
  return lines;
}

function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function highlightAnchors(html, threads, showResolved) {
  for (const t of threads) {
    if (t.detached) continue;
    if (t.resolved && !showResolved) continue;
    const needle = (t.anchorText || "").trim();
    if (!needle) continue;
    const re = new RegExp(`(${escapeRegExp(needle)})`, "");
    const cls = t.resolved ? "anchor-mark resolved" : "anchor-mark";
    const wrapped = `<mark data-thread-id="${t.id}" class="${cls}">$1</mark>`;
    if (html.includes(needle)) {
      html = html.replace(re, wrapped);
    }
  }
  return html;
}

function contextAroundSelection(full, selected) {
  const idx = full.indexOf(selected);
  if (idx < 0) return { prefix: "", suffix: "" };
  const p0 = Math.max(0, idx - 40);
  const s1 = Math.min(full.length, idx + selected.length + 40);
  return {
    prefix: full.slice(p0, idx),
    suffix: full.slice(idx + selected.length, s1),
  };
}
