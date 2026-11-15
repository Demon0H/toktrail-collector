const $=id=>document.getElementById(id), esc=x=>String(x??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
let records=[],isDemo=false,days=1,descending=true;
const format=n=>n.toLocaleString('en-US');
function day(t){return new Date(new Date(t).getTime()+8*3600000).toISOString().slice(0,10);}
const units=value=>{const n=Number(value);return n>=1e8?(n/1e8).toFixed(2)+' 亿':n>=1e4?(n/1e4).toFixed(2)+' 万':format(value);};
async function api(path,method='GET',data){const res=await fetch(path,{method,headers:{'Content-Type':'application/json',Authorization:'Bearer '+$('token').value.trim()},body:data===undefined?undefined:JSON.stringify(data)});const b=await res.json();if(!res.ok)throw Error(b.error||'请求失败');return b;}
function feedback(text,error=false){$('feedback').textContent=text;$('feedback').classList.toggle('error',error);}
async function action(button,fn){button.disabled=true;try{await fn();}catch(e){feedback(e.message,true);}finally{button.disabled=false;}}
function demo(){const names=['Demon0H','林间有风','PixelNomad','阿川','Momo','夜航船','山海之间','Vincent','小满','CodeWanderer'];const tools=['Codex','Claude Code','Qwen Code','Kimi CLI'];return names.flatMap((name,i)=>Array.from({length:14},(_,j)=>({id:'demo'+i,name,city:['杭州','上海','北京','深圳','成都'][i%5],date:new Date(Date.now()-j*86400000).toISOString(),tool:tools[i%4],model:['gpt-5.3-codex','claude-sonnet-4-5','qwen3-coder','kimi-k2'][i%4],input:String(Math.round(740000/(i+1)*(1+((i+j*3)%7)/10))),output:String(Math.round(120000/(i+1))),cacheRead:String(Math.round(580000/(i+1))),cacheWrite:'0'})));}
function options(){for(const [id,key,label] of [['tool','tool','全部工具'],['model','model','全部模型']]){$(id).innerHTML=`<option value="">${label}</option>`+[...new Set(records.map(r=>r[key]))].sort().map(v=>`<option value="${esc(v)}">${esc(v)}</option>`).join('');}}
function render(){
 const today=day(Date.now()),start=days?day(Date.now()-(days-1)*86400000):'0000';const cache=$('cache').checked,q=$('search').value.trim().toLowerCase();let filtered=records.filter(r=>day(r.date)>=start&&day(r.date)<=today&&(!$('tool').value||r.tool===$('tool').value)&&(!$('model').value||r.model===$('model').value));
 const groups=new Map();for(const r of filtered){if(!groups.has(r.id))groups.set(r.id,{...r,input:0n,output:0n,cached:0n,total:0n,unknown:0,tools:new Map()});let g=groups.get(r.id);const fields=['input','output',...(cache?['cacheRead','cacheWrite']:[])];if(fields.some(k=>r[k]===null||r[k]===undefined)){g.unknown++;continue;}const n=k=>BigInt(r[k]||0);g.input+=n('input');g.output+=n('output');g.cached+=cache?n('cacheRead')+n('cacheWrite'):0n;const sum=n('input')+n('output')+(cache?n('cacheRead')+n('cacheWrite'):0n);g.total+=sum;const key=r.tool+' / '+r.model;g.tools.set(key,(g.tools.get(key)||0n)+sum);}
 const all=[...groups.values()].sort((a,b)=>a.total===b.total?0:a.total>b.total?-1:1);all.forEach((g,i)=>g.rank=i+1);let list=all.filter(g=>(g.name+' '+g.city).toLowerCase().includes(q));if(!descending)list.reverse();
 $('total').textContent=units(all.reduce((s,g)=>s+g.total,0n));$('people').textContent=format(all.length);$('tools').textContent=new Set(filtered.map(r=>r.tool)).size;$('count').textContent=`${list.length} 位参与者${isDemo?' · 演示数据':''}`;
 $('board-title').textContent=({1:'今日榜',7:'近 7 天榜',30:'近 30 天榜',0:'总榜'})[days];$('date-label').textContent=today;$('counting-rule').textContent=cache?'计数口径：输入 + 缓存读取 + 缓存写入 + 输出；时区 UTC+8。':'计数口径：输入 + 输出（不含缓存）；时区 UTC+8。';
 $('status').textContent=isDemo?'演示模式 · 以下为虚构数据，不参与真实排名':`真实社区数据 · ${cache?'含缓存读写':'仅输入与输出'} · 未知计数记录不计入总量`;
 $('rows').innerHTML=list.map(g=>{const top=[...g.tools].sort((a,b)=>a[1]===b[1]?0:a[1]>b[1]?-1:1)[0]?.[0]||'未知计数';const sep=top.indexOf(' / ');return `<tr class="rank-${g.rank}"><td><span class="rank-badge">${g.rank}</span></td><td><div class="person"><span class="avatar">${esc([...g.name].slice(0,2).join(''))}</span><div><strong>${esc(g.name)}</strong><small class="mobile-city">${esc(g.city||'未填写城市')}</small></div></div></td><td class="city-col">${esc(g.city||'—')}</td><td class="agent-col">${esc(sep<0?top:top.slice(0,sep))}</td><td class="model-col">${esc(sep<0?'':top.slice(sep+3))}</td><td class="distribution-col"><div class="bar" aria-label="输入 ${format(g.input)}，输出 ${format(g.output)}，缓存 ${format(g.cached)}"><span data-width="${g.total?Number(g.input*10000n/g.total)/100:0}"></span><span class="output" data-width="${g.total?Number(g.output*10000n/g.total)/100:0}"></span><span class="cached" data-width="${g.total?Number(g.cached*10000n/g.total)/100:0}"></span></div></td><td class="value" title="${format(g.total)} Tokens">${units(g.total)}<small>TOKENS${g.unknown?' · 有未知计数':''}</small></td></tr>`;}).join('');document.querySelectorAll('[data-width]').forEach(el=>el.style.width=el.dataset.width+'%');$('empty').hidden=!!list.length;$('live').classList.toggle('selected',!isDemo);$('demo').classList.toggle('selected',isDemo);
}
async function load(){try{const b=await api('/api/records');records=b.records;isDemo=false;options();render();}catch(e){$('status').textContent='加载失败：'+e.message+'，点击真实榜单重试';}}
document.querySelectorAll('[data-days]').forEach(b=>b.onclick=()=>{days=Number(b.dataset.days);document.querySelectorAll('[data-days]').forEach(x=>x.classList.toggle('selected',x===b));render();});
for(const id of ['tool','model','cache','search'])$(id).addEventListener('input',render);
$('sort').onclick=()=>{descending=!descending;$('sort').textContent='Token 用量 '+(descending?'↓':'↑');render();};$('demo').onclick=()=>{records=demo();isDemo=true;options();render();};$('live').onclick=load;
$('register').onsubmit=e=>{e.preventDefault();action(e.submitter,async()=>{
 const f=new FormData(e.target),name=String(f.get('name')||'').trim();let identity;
 if(!$('token').value.trim()){identity=await api('/api/members','POST',{name,public:f.has('public')});$('token').value=identity.token;$('city').textContent=identity.city||'暂时无法识别';}
 else {identity=await api('/api/me');if(identity.name!==name)throw Error('已有密钥属于 '+identity.name+'，请使用相同昵称，或清空密钥创建新身份。');}
 const b=await api('/api/enrollment','POST',{});
 const prompt=`请阅读 https://tokenrank.demonh.site/agent-setup.md，帮我接入 DH燃源榜。昵称：${JSON.stringify(name)}；一次性接入码：${b.ticket}（15分钟有效）。请识别系统和架构，下载安装最新专用采集器并校验 SHA-256，使用 setup --ticket 接入码 --all --install，采集本机所有受支持 AI 工具的用量，包含已有历史及后续新增工具，执行首次同步并验证定时任务。只连接该站点，公开状态由我在网页选择；不得输出或提交长期密钥。先展示自动发现的工具目录，若已有配置先验证 sync，避免覆盖身份。`;
 $('agent-prompt').textContent=prompt;
 if(navigator.clipboard){await navigator.clipboard.writeText(prompt);feedback('专属配置已复制。请保存右侧参与密钥，用于撤回公开和删除；接入码仅能使用一次。');e.submitter.textContent='已复制，粘贴给 Agent';}
 else {feedback('接入配置已生成。当前 HTTP 页面不能使用系统剪贴板，请手动选中上面的完整说明复制，或使用 HTTPS 入口。');}
 });};
$('copy').onclick=()=>action($('copy'),async()=>{if(!$('token').value)throw Error('请先创建身份或输入密钥');await navigator.clipboard.writeText($('token').value);feedback('密钥已复制，请妥善保存。');});
$('import').onclick=()=>action($('import'),async()=>{if(!$('token').value.trim())throw Error('请先创建身份或输入密钥');const f=$('file').files[0];if(!f)throw Error('请先选择导出的 JSON 文件');if(f.size>12*1024*1024)throw Error('文件超过 12 MB');let b;try{b=JSON.parse(await f.text());}catch{throw Error('JSON 文件无法解析，请重新导出');}const r=await api('/api/import','POST',b);feedback(`导入成功：新增 ${r.added} 条，跳过 ${r.duplicates} 条重复记录。公开参与后可在榜单查看。`);await load();});
for(const [id,pub] of [['publish',true],['hide',false]])$(id).onclick=()=>action($(id),async()=>{await api('/api/me','PATCH',{public:pub});feedback(pub?'已公开参与，昵称、城市和用量将展示在榜单。':'已撤回公开，数据保留供你后续参与。');await load();});
$('delete').onclick=()=>action($('delete'),async()=>{if(!confirm('确定永久删除这个参与身份和全部用量？此操作无法撤销。'))return;await api('/api/me','DELETE');$('token').value='';feedback('身份与用量已删除。');await load();});
load();

$('platform').onchange=()=>{$('download').href='https://github.com/Demon0H/toktrail-collector/releases/latest/download/dh-tokenrank-'+$('platform').value;};

api('/api/location').then(b=>{$('city').textContent=b.city||'暂时无法识别（不影响接入）';}).catch(()=>{$('city').textContent='暂时无法识别（不影响接入）';});
