/* global Alpine, marked */

function shell() {
  return {
    markdown: "",
    threads: [],
    renderedHtml: "",
    fileMeta: { name: "", path: "", relPath: "" },
    projectTree: { root: "", sections: [] },
    _treeRefreshTimer: null,
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
    detachedPanelOpen: false,

    get visibleThreads() {
      return this.threads.filter((t) => !t.detached);
    },

    get detachedThreads() {
      return this.threads.filter((t) => t.detached);
    },

    get detachedCount() {
      return this.detachedThreads.length;
    },

    get commentCount() {
      return this.visibleThreads.length;
    },

    get canNavigateComments() {
      return this.viewMode === "preview" && this.unresolvedNavList().length > 0;
    },

    get activeThread() {
      return this.activeThreadId
        ? this.threads.find((t) => t.id === this.activeThreadId) || null
        : null;
    },

    get diffRangeLabel() {
      const a = this.history.left;
      const b = this.history.right;
      if (!a || !b) return "";
      return `${shortId(a)} → ${shortId(b)}`;
    },

    init() {
      this.fetchFileMeta();
      this.fetchProjectTree();
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

    async fetchProjectTree() {
      try {
        const res = await fetch("/api/project/tree");
        if (res.ok) {
          this.projectTree = await res.json();
        }
      } catch (_) {
        /* ignore */
      }
    },

    scheduleProjectTreeRefresh() {
      if (this._treeRefreshTimer) clearTimeout(this._treeRefreshTimer);
      this._treeRefreshTimer = setTimeout(() => {
        this._treeRefreshTimer = null;
        this.fetchProjectTree();
      }, 400);
    },

    resetHistoryForFileSwitch() {
      this.historyOpen = false;
      this.snapshots = [];
      this.diffLines = [];
      this.history = {
        left: "",
        right: "",
        focusIndex: 0,
        hunks: [],
        accept: {},
      };
      this.closeThread();
    },

    async selectFile(relPath) {
      if (relPath === this.fileMeta.relPath) return;
      const res = await fetch("/api/file/select", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ path: relPath }),
      });
      if (!res.ok) {
        alert(await res.text());
      }
    },

    connectWS() {
      const proto = window.location.protocol === "https:" ? "wss" : "ws";
      this.ws = new WebSocket(`${proto}://${window.location.host}/ws`);
      this.ws.onmessage = (ev) => {
        const msg = JSON.parse(ev.data);
        if (msg.type === "active_file") {
          this.fileMeta = {
            name: msg.name || "",
            path: msg.path || "",
            relPath: msg.relPath || "",
          };
          this.resetHistoryForFileSwitch();
          this.fetchProjectTree();
        } else if (msg.type === "file_update") {
          this.markdown = msg.content;
          this.render();
        } else if (msg.type === "threads_update") {
          this.threads = msg.threads || [];
          if (this.detachedCount === 0) this.detachedPanelOpen = false;
          this.scheduleProjectTreeRefresh();
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
      const norm = (m) => (m && m.role ? String(m.role) : "").toLowerCase().trim();
      const roles = new Set();
      for (const m of msgs) {
        const r = norm(m);
        if (r) roles.add(r);
      }
      if (roles.size === 0) return "Thread";
      if (roles.size === 1) {
        const only = roles.values().next().value;
        if (only === "user") return "You";
        if (only === "agent") return "Agent";
        return "Thread";
      }
      return "Thread";
    },

    messageRoleLabel(m) {
      const r = (m && m.role ? String(m.role) : "").toLowerCase().trim();
      if (r === "user") return "You";
      if (r === "agent") return "Agent";
      return "Comment";
    },

    messageRoleRowClass(m) {
      const r = (m && m.role ? String(m.role) : "").toLowerCase().trim();
      if (r === "user") return "bubble-msg-row--user";
      if (r === "agent") return "bubble-msg-row--agent";
      return "bubble-msg-row--unknown";
    },

    formatMessageTime(ts) {
      if (ts == null || ts === "") return "";
      const n = typeof ts === "number" ? ts : Number(ts);
      if (!Number.isFinite(n) || n <= 0) return "";
      const d = new Date(n);
      if (Number.isNaN(d.getTime())) return "";
      return d.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit",
      });
    },

    messageTimeISO(ts) {
      if (ts == null || ts === "") return "";
      const n = typeof ts === "number" ? ts : Number(ts);
      if (!Number.isFinite(n) || n <= 0) return "";
      const d = new Date(n);
      if (Number.isNaN(d.getTime())) return "";
      return d.toISOString();
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
        return;
      }
      const rr = rail.getBoundingClientRect();
      this.bubble = { open: true, top: Math.max(8, Math.min(48, rr.height * 0.15)) };
    },

    threadPreview(t) {
      const s = ((t && t.anchorText) || "").trim().replace(/\s+/g, " ");
      return s.length > 52 ? s.slice(0, 49) + "…" : s;
    },

    threadHasMessages(thread) {
      if (!thread) return false;
      return (thread.thread || []).some((m) => (m.body || "").trim() !== "");
    },

    parentMessage(thread) {
      const msgs = ((thread && thread.thread) || []).filter((m) => (m.body || "").trim() !== "");
      return msgs.length ? msgs[0] : null;
    },

    replyMessages(thread) {
      const msgs = ((thread && thread.thread) || []).filter((m) => (m.body || "").trim() !== "");
      return msgs.length > 1 ? msgs.slice(1) : [];
    },

    messageBody(msg) {
      return msg && msg.body ? String(msg.body) : "";
    },

    messageAuthor(msg) {
      const r = (msg && msg.role ? String(msg.role) : "").toLowerCase().trim();
      if (r === "user") return "You";
      if (r === "agent") return "Agent";
      return "Comment";
    },

    messageRelativeTime(msg) {
      if (!msg || msg.ts == null || msg.ts === "") return "";
      const n = typeof msg.ts === "number" ? msg.ts : Number(msg.ts);
      if (!Number.isFinite(n) || n <= 0) return "";
      const deltaSeconds = Math.max(0, Math.floor((Date.now() - n) / 1000));
      if (deltaSeconds < 60) return "now";
      if (deltaSeconds < 3600) return `${Math.floor(deltaSeconds / 60)}m ago`;
      if (deltaSeconds < 86400) return `${Math.floor(deltaSeconds / 3600)}h ago`;
      return `${Math.floor(deltaSeconds / 86400)}d ago`;
    },

    avatarLetter(msg, thread, isLarge) {
      const body = this.messageBody(msg).trim();
      if (body) return this.messageAuthor(msg).charAt(0).toUpperCase();
      if (isLarge && thread) return "T";
      return "C";
    },

    avatarClass(msg, isLarge) {
      const r = (msg && msg.role ? String(msg.role) : "").toLowerCase().trim();
      if (r === "user") return isLarge ? "thread-avatar-user-lg" : "thread-avatar-user";
      if (r === "agent") return isLarge ? "thread-avatar-agent-lg" : "thread-avatar-agent";
      return isLarge ? "thread-avatar-neutral-lg" : "thread-avatar-neutral";
    },

    formatText(input) {
      const escaped = escapeHtml(input || "");
      const withMentions = escaped.replace(/(^|\s)@([a-zA-Z0-9._-]+)/g, '$1<span class="mention">@$2</span>');
      return withMentions.replace(/\n/g, "<br>");
    },

    async deleteThread(threadId) {
      if (!threadId) return;
      const res = await fetch("/api/threads/delete", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: threadId }),
      });
      if (!res.ok) {
        alert(await res.text());
        return;
      }
      if (this.activeThreadId === threadId) {
        this.activeThreadId = null;
        this.bubble.open = false;
      }
      delete this.drafts[threadId];
    },

    async closeThread() {
      const id = this.activeThreadId;
      const t = id ? this.threads.find((x) => x.id === id) : null;
      const hasMsgs = this.threadHasMessages(t);
      const draft = id ? (this.drafts[id] || "").trim() : "";

      this.activeThreadId = null;
      this.bubble.open = false;

      if (id && t && !hasMsgs && !draft) {
        try {
          await fetch("/api/threads/delete", {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({ id }),
          });
        } catch (_) {
          /* ignore */
        }
      }
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
      let left = rect.left - rr.left + rect.width / 2;
      /* .selection-popover uses translateX(-50%); keep chip inside row */
      const halfChip = 80;
      if (rr.width <= halfChip * 2) {
        left = rr.width / 2;
      } else {
        left = Math.max(halfChip, Math.min(rr.width - halfChip, left));
      }
      this.toolbar = {
        show: true,
        top: rect.top - rr.top - 42,
        left,
      };
      const full = this.markdown;
      const ctx = contextAroundSelection(full, text);
      this.sel = { text, prefix: ctx.prefix, suffix: ctx.suffix };
    },

    async addCommentFromSelection() {
      const id = crypto.randomUUID();
      const span = findMarkdownSpan(this.markdown, this.sel.text);
      let anchorText = this.sel.text;
      let anchor = undefined;
      if (span) {
        anchorText = this.markdown.slice(span.start, span.end);
        const p0 = Math.max(0, span.start - 40);
        const s1 = Math.min(this.markdown.length, span.end + 40);
        anchor = {
          startOffset: span.start,
          endOffset: span.end,
          prefix: this.markdown.slice(p0, span.start),
          suffix: this.markdown.slice(span.end, s1),
        };
      }
      const body = { id, anchorText, anchor };
      this._pendingOpenId = id;
      const res = await fetch("/api/threads/upsert", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        this._pendingOpenId = null;
        alert(await res.text());
        return;
      }
      this.toolbar.show = false;
      window.getSelection()?.removeAllRanges();
    },

    async reattachThreadFromSelection(threadId) {
      const span = findMarkdownSpan(this.markdown, this.sel.text);
      let anchorText = this.sel.text;
      let anchor = undefined;
      if (span) {
        anchorText = this.markdown.slice(span.start, span.end);
        const p0 = Math.max(0, span.start - 40);
        const s1 = Math.min(this.markdown.length, span.end + 40);
        anchor = {
          startOffset: span.start,
          endOffset: span.end,
          prefix: this.markdown.slice(p0, span.start),
          suffix: this.markdown.slice(span.end, s1),
        };
      }
      this._pendingOpenId = threadId;
      const res = await fetch("/api/threads/upsert", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: threadId, anchorText, anchor }),
      });
      if (!res.ok) {
        this._pendingOpenId = null;
        alert(await res.text());
        return;
      }
      this.toolbar.show = false;
      this.detachedPanelOpen = false;
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
      out +=
        "When editing the document, keep quoted anchor text intact when possible. " +
        "If a passage moves, re-attach the same thread id (see CLI: comments reattach --thread <id> --anchor \"…\").\n\n";
      let i = 1;
      for (const t of this.visibleThreads) {
        if (t.resolved && !this.showResolved) continue;
        out += `[Thread ${i}] id: ${t.id}\n`;
        out += `    Anchor: ${JSON.stringify(t.anchorText)}\n`;
        for (const m of t.thread || []) {
          out += `> ${m.role}: ${m.body}\n`;
        }
        const d = (this.drafts[t.id] || "").trim();
        if (d) out += `> (draft): ${d}\n`;
        out += "\n";
        i++;
      }
      const detached = this.detachedThreads.filter((t) => !t.resolved || this.showResolved);
      if (detached.length) {
        out += "\n--- [DETACHED: anchor text not found in file — re-attach after edits] ---\n\n";
        for (const t of detached) {
          out += `[Detached] id: ${t.id} resolved=${!!t.resolved}\n`;
          out += `    Last anchor: ${JSON.stringify(t.anchorText)}\n`;
          for (const m of t.thread || []) {
            out += `> ${m.role}: ${m.body}\n`;
          }
          out += "\n";
        }
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

function stripBasicMarkdownForMatch(s) {
  if (!s) return "";
  return s
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/`([^`]+)`/g, "$1");
}

/** Lets <strong>, etc. appear between words when matching rendered HTML. */
function wordsBridgePatternSource(plainChunk) {
  const words = plainChunk.trim().split(/\s+/).filter(Boolean);
  if (!words.length) return null;
  return words.map(escapeRegExp).join("[\\s\\S]*?");
}

/** Paragraph chunks split like markdown blank lines; used for preview highlighting. */
function anchorParagraphBlocks(needleMd) {
  const trimmed = (needleMd || "").trim();
  if (!trimmed) return [];
  return trimmed
    .split(/\n\s*\n/)
    .map((p) => stripBasicMarkdownForMatch(p.trim()))
    .filter(Boolean);
}

/**
 * One <mark> per paragraph. A single mark must not wrap </p>…<p> — that is invalid HTML and breaks selection styling.
 */
function highlightSequentialParagraphMarks(html, blocks, threadId, cls) {
  let cur = html;
  let from = 0;
  const open = () => `<mark data-thread-id="${threadId}" class="${cls}">`;
  const close = `</mark>`;
  for (const block of blocks) {
    const src = wordsBridgePatternSource(block);
    if (!src) return null;
    const re = new RegExp(`(${src})`);
    const sub = cur.slice(from);
    const m = sub.match(re);
    if (!m || m.index === undefined) return null;
    const absStart = from + m.index;
    const full = m[0];
    const absEnd = absStart + full.length;
    const o = open();
    cur = cur.slice(0, absStart) + o + full + close + cur.slice(absEnd);
    from = absStart + o.length + full.length + close.length;
  }
  return cur;
}

/**
 * When markdown anchor text is not a literal substring of HTML (e.g. **bold**). Multi-paragraph anchors use highlightSequentialParagraphMarks instead.
 */
function anchorHtmlPattern(needleMd) {
  const trimmed = (needleMd || "").trim();
  if (!trimmed) return null;
  const blocks = anchorParagraphBlocks(needleMd);
  if (blocks.length >= 2) return null;
  const one = blocks.length === 1 ? blocks[0] : stripBasicMarkdownForMatch(trimmed);
  const src = wordsBridgePatternSource(one);
  if (src) return new RegExp(`(${src})`, "");
  return null;
}

function highlightAnchors(html, threads, showResolved) {
  for (const t of threads) {
    if (t.detached) continue;
    if (t.resolved && !showResolved) continue;
    const needle = (t.anchorText || "").trim();
    if (!needle) continue;
    const cls = t.resolved ? "anchor-mark resolved" : "anchor-mark";
    const wrapped = `<mark data-thread-id="${t.id}" class="${cls}">$1</mark>`;
    const blocks = anchorParagraphBlocks(needle);
    if (blocks.length >= 2) {
      const next = highlightSequentialParagraphMarks(html, blocks, t.id, cls);
      if (next) {
        html = next;
        continue;
      }
    }
    if (html.includes(needle)) {
      const re = new RegExp(`(${escapeRegExp(needle)})`, "");
      html = html.replace(re, wrapped);
    } else {
      const alt = anchorHtmlPattern(needle);
      if (alt && alt.test(html)) {
        html = html.replace(alt, wrapped);
      }
    }
  }
  return html;
}

/** DOM selections often use a single \\n between blocks; markdown uses blank lines (\\n\\n). */
function expandCrossBlockNewlines(s) {
  let cur = s;
  for (;;) {
    const next = cur.replace(/([^\n])\n([^\n])/g, "$1\n\n$2");
    if (next === cur) break;
    cur = next;
  }
  return cur;
}

/**
 * Maps the preview’s visible text (what getSelection().toString() returns) onto markdown byte offsets.
 * Skips **bold** and `code` delimiters so "review" matches **review** in the source.
 */
function mdVisiblePlainWithMap(markdown) {
  const plainChars = [];
  const mdIdx = [];
  const n = markdown.length;
  let i = 0;
  while (i < n) {
    if (markdown[i] === "\r") {
      i++;
      continue;
    }
    if (markdown.startsWith("**", i)) {
      i += 2;
      while (i < n && !markdown.startsWith("**", i)) {
        if (markdown[i] === "\r") {
          i++;
          continue;
        }
        plainChars.push(markdown[i]);
        mdIdx.push(i);
        i++;
      }
      if (markdown.startsWith("**", i)) i += 2;
      continue;
    }
    if (markdown[i] === "`") {
      i++;
      while (i < n && markdown[i] !== "`") {
        if (markdown[i] === "\r") {
          i++;
          continue;
        }
        plainChars.push(markdown[i]);
        mdIdx.push(i);
        i++;
      }
      if (markdown[i] === "`") i++;
      continue;
    }
    plainChars.push(markdown[i]);
    mdIdx.push(i);
    i++;
  }
  return { plain: plainChars.join(""), mdIdx };
}

function findMarkdownSpan(markdown, selectedRaw) {
  if (!markdown || selectedRaw == null || selectedRaw === "") return null;
  const sel = String(selectedRaw).replace(/\r\n/g, "\n").trim();
  if (!sel) return null;

  const { plain, mdIdx } = mdVisiblePlainWithMap(markdown);
  const tryInPlain = (candidate) => {
    if (!candidate) return null;
    const p = plain.indexOf(candidate);
    if (p < 0) return null;
    const endPlain = p + candidate.length;
    if (endPlain > mdIdx.length || endPlain <= 0) return null;
    return { start: mdIdx[p], end: mdIdx[endPlain - 1] + 1 };
  };

  let span = tryInPlain(sel);
  if (span) return span;
  const expanded = expandCrossBlockNewlines(sel);
  if (expanded !== sel) {
    span = tryInPlain(expanded);
    if (span) return span;
  }

  /* Last resort: raw markdown substring (plain selections that match source literally). */
  const rawTry = (candidate) => {
    const start = markdown.indexOf(candidate);
    if (start < 0) return null;
    return { start, end: start + candidate.length };
  };
  span = rawTry(sel);
  if (span) return span;
  if (expanded !== sel) return rawTry(expanded);
  return null;
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

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}
