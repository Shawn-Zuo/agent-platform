package web

const indexHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Agent Platform 可视化</title>
<style>
  :root {
    --bg: #0f1117; --panel: #1a1d27; --border: #2a2e3a; --text: #e4e6eb;
    --muted: #8a90a0; --accent: #6c8cff; --ok: #3fb950; --err: #f85149;
    --run: #d29922; --planner:#a371f7; --executor:#6c8cff; --rag:#3fb950; --memory:#d29922;
  }
  * { box-sizing: border-box; }
  body { margin:0; font-family: -apple-system, "Segoe UI", Roboto, "PingFang SC", sans-serif;
    background: var(--bg); color: var(--text); }
  header { padding: 16px 24px; border-bottom: 1px solid var(--border); display:flex; align-items:center; gap:16px; }
  header h1 { font-size: 18px; margin:0; font-weight:600; }
  header .badge { font-size:12px; color:var(--muted); }
  .wrap { display:grid; grid-template-columns: 1fr 320px; gap:16px; padding:16px 24px; }
  .col { display:flex; flex-direction:column; gap:16px; }
  .card { background: var(--panel); border:1px solid var(--border); border-radius:10px; padding:16px; }
  .card h2 { font-size:13px; text-transform:uppercase; letter-spacing:.5px; color:var(--muted); margin:0 0 12px; }
  .runbar { display:flex; gap:8px; }
  select, input, button { font: inherit; border-radius:8px; border:1px solid var(--border);
    background:#12141c; color:var(--text); padding:9px 12px; }
  input { flex:1; }
  button { background: var(--accent); border-color: var(--accent); color:#fff; cursor:pointer; font-weight:600; }
  button:disabled { opacity:.5; cursor:not-allowed; }
  .presets { display:flex; flex-wrap:wrap; gap:6px; margin-top:10px; }
  .presets .chip { font-size:12px; padding:5px 10px; background:#12141c; border:1px solid var(--border);
    border-radius:20px; cursor:pointer; color:var(--muted); }
  .presets .chip:hover { color:var(--text); border-color:var(--accent); }
  .steps { display:flex; flex-direction:column; gap:10px; }
  .step { border:1px solid var(--border); border-radius:10px; padding:12px 14px; background:#12141c;
    border-left:4px solid var(--border); transition:.2s; }
  .step.running { border-left-color: var(--run); box-shadow:0 0 0 1px var(--run) inset; }
  .step.done { border-left-color: var(--ok); }
  .step.error { border-left-color: var(--err); }
  .step .top { display:flex; align-items:center; gap:8px; margin-bottom:6px; }
  .step .id { font-weight:700; font-size:13px; }
  .tag { font-size:11px; padding:2px 8px; border-radius:12px; font-weight:600; }
  .tag.executor{background:rgba(108,140,255,.15);color:var(--executor);}
  .tag.rag{background:rgba(63,185,80,.15);color:var(--rag);}
  .tag.memory{background:rgba(210,153,34,.15);color:var(--memory);}
  .tag.planner{background:rgba(163,113,247,.15);color:var(--planner);}
  .step .desc { font-size:13px; color:var(--text); margin-bottom:6px; }
  .step .meta { font-size:12px; color:var(--muted); font-family:ui-monospace, monospace; }
  .step .deps { font-size:11px; color:var(--muted); margin-top:4px; }
  .step .out { margin-top:8px; font-size:12px; background:#0b0d13; border:1px solid var(--border);
    border-radius:6px; padding:8px; white-space:pre-wrap; word-break:break-word; font-family:ui-monospace,monospace; max-height:180px; overflow:auto; }
  .status-dot { width:8px; height:8px; border-radius:50%; background:var(--muted); }
  .status-dot.running{background:var(--run); animation:pulse 1s infinite;}
  .status-dot.done{background:var(--ok);}
  .status-dot.error{background:var(--err);}
  @keyframes pulse { 0%,100%{opacity:1;}50%{opacity:.3;} }
  .tool, .memrow { font-size:12px; padding:8px 0; border-bottom:1px solid var(--border); }
  .tool:last-child,.memrow:last-child{border-bottom:none;}
  .tool b{color:var(--accent);} .memrow b{color:var(--memory);}
  .muted{color:var(--muted);} .empty{color:var(--muted);font-size:13px;padding:8px 0;}
  #goalbanner{font-size:13px;color:var(--muted);margin-bottom:10px;}
</style>
</head>
<body>
<header>
  <h1>🤖 Agent Platform</h1>
  <span class="badge">Workflow 执行可视化</span>
  <span class="badge" id="conn"></span>
</header>
<div class="wrap">
  <div class="col">
    <div class="card">
      <h2>运行工作流</h2>
      <div class="runbar">
        <input id="goal" placeholder="输入一个目标 (Goal)…" />
        <button id="run">运行</button>
      </div>
      <div class="presets" id="presets"></div>
    </div>
    <div class="card">
      <h2>执行步骤 (DAG)</h2>
      <div id="goalbanner"></div>
      <div class="steps" id="steps"><div class="empty">尚未运行。输入目标并点击「运行」。</div></div>
    </div>
  </div>
  <div class="col">
    <div class="card">
      <h2>可用工具</h2>
      <div id="tools"><div class="empty">加载中…</div></div>
    </div>
    <div class="card">
      <h2>Memory Store</h2>
      <div id="memory"><div class="empty">空</div></div>
    </div>
  </div>
</div>
<script>
const $ = s => document.querySelector(s);
const stepsEl = $('#steps'), goalEl = $('#goal'), runBtn = $('#run'), connEl = $('#conn');
let es = null;

const PRESETS = [
  "Calculate 123 multiplied by 456, then store the result in memory under key 'product'",
  "Search the knowledge base to learn about RAG and agents, then summarize what you found",
];
const pc = $('#presets');
PRESETS.forEach(p => {
  const c = document.createElement('span'); c.className='chip'; c.textContent = p.slice(0,42)+'…';
  c.title = p; c.onclick = () => { goalEl.value = p; }; pc.appendChild(c);
});

function esc(s){ return String(s).replace(/[&<>]/g, m=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[m])); }

function renderPlan(plan){
  stepsEl.innerHTML = '';
  (plan.Steps||[]).forEach(st => {
    const d = document.createElement('div');
    d.className = 'step'; d.id = 'step-'+st.ID;
    const type = (st.AgentType||'executor');
    const deps = (st.DependsOn||[]).length ? '依赖: '+st.DependsOn.join(', ') : '无依赖';
    const input = st.ToolInput ? JSON.stringify(st.ToolInput) : '{}';
    d.innerHTML =
      '<div class="top"><span class="status-dot" id="dot-'+st.ID+'"></span>'+
      '<span class="id">'+esc(st.ID)+'</span>'+
      '<span class="tag '+type+'">'+type+'</span></div>'+
      '<div class="desc">'+esc(st.Description||'')+'</div>'+
      '<div class="meta">🔧 '+esc(st.ToolName||'-')+'  '+esc(input)+'</div>'+
      '<div class="deps">'+esc(deps)+'</div>'+
      '<div class="out" id="out-'+st.ID+'" style="display:none"></div>';
    stepsEl.appendChild(d);
  });
}

function setState(id, state){
  const s = document.getElementById('step-'+id), dot = document.getElementById('dot-'+id);
  if(!s) return;
  s.classList.remove('running','done','error'); s.classList.add(state);
  if(dot){ dot.classList.remove('running','done','error'); dot.classList.add(state); }
}

function showOut(id, text, isErr){
  const o = document.getElementById('out-'+id);
  if(!o) return;
  o.style.display='block'; o.textContent = text;
  o.style.color = isErr ? 'var(--err)' : '';
}

function run(){
  const goal = goalEl.value.trim();
  if(!goal) return;
  if(es) es.close();
  stepsEl.innerHTML = '<div class="empty">正在规划 (Planner)…</div>';
  $('#goalbanner').textContent = '🎯 ' + goal;
  runBtn.disabled = true; connEl.textContent = '● 运行中';
  es = new EventSource('/api/run?goal='+encodeURIComponent(goal));
  es.onmessage = e => {
    const ev = JSON.parse(e.data);
    if(ev.kind==='plan') renderPlan(ev.plan);
    else if(ev.kind==='step_start') setState(ev.step.ID,'running');
    else if(ev.kind==='step_done'){
      setState(ev.step.ID,'done');
      if(ev.result) showOut(ev.step.ID, ev.result.Output||'(无输出)', ev.result.IsError);
    }
    else if(ev.kind==='error'){
      if(ev.step) { setState(ev.step.ID,'error'); showOut(ev.step.ID, ev.detail, true); }
      else stepsEl.innerHTML = '<div class="empty" style="color:var(--err)">错误: '+esc(ev.detail)+'</div>';
    }
    else if(ev.kind==='workflow_done'){ connEl.textContent='✓ 完成 ('+esc(ev.detail)+')'; }
  };
  es.addEventListener('end', () => { es.close(); runBtn.disabled=false; loadMemory(); });
  es.onerror = () => { es.close(); runBtn.disabled=false; connEl.textContent='● 已断开'; };
}
runBtn.onclick = run;
goalEl.addEventListener('keydown', e => { if(e.key==='Enter') run(); });

async function loadTools(){
  try{
    const t = await (await fetch('/api/tools')).json();
    $('#tools').innerHTML = (t||[]).map(x =>
      '<div class="tool"><b>'+esc(x.name)+'</b><div class="muted">'+esc(x.description)+'</div></div>'
    ).join('') || '<div class="empty">无</div>';
  }catch(e){ $('#tools').innerHTML='<div class="empty">加载失败</div>'; }
}
async function loadMemory(){
  try{
    const m = await (await fetch('/api/memory')).json();
    const keys = Object.keys(m||{});
    $('#memory').innerHTML = keys.length ? keys.map(k =>
      '<div class="memrow"><b>'+esc(k)+'</b>: '+esc(m[k])+'</div>'
    ).join('') : '<div class="empty">空</div>';
  }catch(e){ $('#memory').innerHTML='<div class="empty">加载失败</div>'; }
}
loadTools(); loadMemory();
</script>
</body>
</html>`
