package main

// reviewHTML is a self-contained review sheet (no server, no external JS). For each F0 word
// it shows the auto-picked "best guess" plus in-band alternates and the rejected candidates
// with reasons. The reviewer approves the pick or overrides it, then clicks "Export decisions"
// to download a decisions JSON to hand back (blocker B-4 review loop).
const reviewHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Zhuwen — F0 image review</title>
<style>
 body{font:15px/1.5 -apple-system,system-ui,sans-serif;margin:0;color:#1a1a1a;background:#faf9f7}
 header{position:sticky;top:0;background:#7a1f1f;color:#fff;padding:14px 22px;z-index:9}
 header b{font-size:18px} header .hint{opacity:.85;font-size:13px}
 header button{float:right;background:#fff;color:#7a1f1f;border:0;border-radius:6px;padding:8px 14px;font-weight:600;cursor:pointer}
 .word{background:#fff;margin:18px 22px;border:1px solid #e6e2dc;border-radius:10px;overflow:hidden}
 .word h2{margin:0;padding:12px 16px;background:#f3efe9;border-bottom:1px solid #e6e2dc;font-size:16px}
 .word h2 .hz{font-size:26px;margin-right:10px} .word h2 .py{color:#7a1f1f;margin-right:8px} .word h2 .en{color:#666;font-weight:400}
 .word .desc{padding:10px 16px;background:#fffdf8;border-bottom:1px solid #efe8dc;color:#3a3a3a;font-size:14px;line-height:1.55}
 .word .desc .q{color:#7a1f1f;font-weight:600} .word .desc .searched{color:#888;font-size:12px;display:block;margin-top:4px}
 .sethdr{margin:26px 22px 4px;font-size:20px;color:#7a1f1f;border-bottom:2px solid #e0b0b0;padding-bottom:4px;text-transform:capitalize}
 .grid{display:flex;flex-wrap:wrap;gap:14px;padding:16px}
 .cand{width:220px;border:2px solid #eee;border-radius:8px;padding:8px;cursor:pointer}
 .cand.best{border-color:#2e7d32;background:#f2f9f2} .cand.sel{border-color:#7a1f1f;box-shadow:0 0 0 2px #7a1f1f33}
 .cand img{width:100%;height:150px;object-fit:cover;border-radius:5px;background:#eee}
 .cand .lic{font-size:12px;color:#2e7d32;font-weight:600;margin-top:6px} .cand .meta{font-size:11px;color:#777;word-break:break-word}
 .cand a{font-size:11px} .badge{display:inline-block;background:#2e7d32;color:#fff;font-size:10px;padding:1px 6px;border-radius:8px}
 .rej{opacity:.6} .rej .lic{color:#b00} .none{padding:16px;color:#b00}
 label.pick{display:block;font-size:12px;margin-top:6px} .reject-word{margin:0 16px 6px;font-size:13px}
 .custom-word{margin:0 16px 14px;font-size:13px} .custom-word input{width:60%;padding:5px 8px;border:1px solid #ccc;border-radius:5px;font-size:13px}
 .custom-word.filled input{border-color:#7a1f1f;background:#fff6f6}
</style></head><body>
<header>
 <button onclick="exportDecisions()">Export decisions ▾</button>
 <b>Zhuwen — Foundations image review</b> &nbsp;<span class="hint">auto-picked best-of-N from Wikimedia Commons · click a card to override the ✓ pick · then Export</span>
</header>
{{range .}}
<div class="sethdr">{{.Set}}</div>
{{range $w := .Words}}
<section class="word" data-word="{{.Simp}}">
 <h2><span class="hz">{{.Simp}}</span><span class="py">{{.Pinyin}}</span><span class="en">{{.En}}</span></h2>
 {{if .Desc}}<div class="desc">{{.Desc}}<span class="searched">🔍 searched Commons for: <span class="q">{{.En}}</span> — pick the image that best represents this story, or paste your own below.</span></div>{{end}}
 {{if .Best}}
 <div class="grid">
   {{with .Best}}
   <div class="cand best sel" data-title="{{.Title}}" onclick="sel(this)">
     <img loading="lazy" src="{{.Thumb}}" alt="">
     <div><span class="badge">✓ best guess</span>{{if .P18}} <span class="badge">P18</span>{{end}}</div>
     <div class="lic">{{.License}} · {{px .}}</div>
     <div class="meta">{{.Author}}</div>
     <a href="{{.DescURL}}" target="_blank">Commons page ↗</a>
     <label class="pick"><input type="radio" name="{{$w.Simp}}" value="{{.Title}}" checked> use this</label>
   </div>
   {{end}}
   {{range .Alts}}
   <div class="cand" data-title="{{.Title}}" onclick="sel(this)">
     <img loading="lazy" src="{{.Thumb}}" alt="">
     <div>{{if .P18}}<span class="badge">P18</span>{{end}}</div>
     <div class="lic">{{.License}} · {{px .}}</div>
     <div class="meta">{{.Author}}</div>
     <a href="{{.DescURL}}" target="_blank">Commons page ↗</a>
     <label class="pick"><input type="radio" name="{{$w.Simp}}" value="{{.Title}}"> use this</label>
   </div>
   {{end}}
 </div>
 {{else}}
 <div class="none">No candidate passed the §8A gate — needs re-query / manual sourcing.</div>
 {{end}}
 <div class="reject-word"><label><input type="radio" name="{{.Simp}}" value="__reject__"> reject all / re-query this word</label></div>
 <div class="custom-word"><label>🔗 use my own: <input type="text" class="custom" placeholder="paste a Commons File:…jpg title or a commons.wikimedia.org page URL" oninput="markCustom(this)"></label> <span style="color:#999">(overrides the pick above when filled)</span></div>
 {{if .Rejects}}
 <details><summary style="margin:0 16px 12px;cursor:pointer;color:#888">Rejected candidates ({{len .Rejects}})</summary>
 <div class="grid">
   {{range .Rejects}}
   <div class="cand rej"><img loading="lazy" src="{{.Thumb}}" alt="">
     <div class="lic">✗ {{.RejectWhy}}</div><div class="meta">{{.License}} · {{px .}}</div>
     <a href="{{.DescURL}}" target="_blank">Commons page ↗</a></div>
   {{end}}
 </div></details>
 {{end}}
 </section>
{{end}}
{{end}}
<script>
function sel(card){
  var sec=card.closest('.word');
  sec.querySelectorAll('.cand').forEach(function(c){c.classList.remove('sel')});
  card.classList.add('sel');
  var r=card.querySelector('input[type=radio]'); if(r) r.checked=true;
}
function markCustom(inp){
  inp.closest('.custom-word').classList.toggle('filled', inp.value.trim()!=='');
}
function exportDecisions(){
  var out=[];
  document.querySelectorAll('.word').forEach(function(sec){
    var w=sec.getAttribute('data-word');
    var custom=sec.querySelector('.custom');
    var cv=custom? custom.value.trim() : '';
    if(cv){ out.push({word:w, decision:cv, custom:true}); return; }
    var r=sec.querySelector('input[type=radio]:checked');
    out.push({word:w, decision: r? r.value : null});
  });
  var blob=new Blob([JSON.stringify(out,null,2)],{type:'application/json'});
  var a=document.createElement('a');
  a.href=URL.createObjectURL(blob); a.download='image-decisions.json'; a.click();
}
</script>
</body></html>`
