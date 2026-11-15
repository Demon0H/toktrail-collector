import {devices} from './devices.mjs';
import {githubAuth} from './auth.mjs';
import {locate} from './geo.mjs';
import http from 'node:http';
import { DatabaseSync } from 'node:sqlite';
import { randomBytes, createHash } from 'node:crypto';
import { readFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const dir = process.env.DATA_DIR || fileURLToPath(new URL('./data', import.meta.url));
mkdirSync(dir,{recursive:true});
const db = new DatabaseSync(`${dir}/dh.sqlite`);
db.exec(`PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS members(id TEXT PRIMARY KEY,name TEXT NOT NULL,city TEXT NOT NULL,secret TEXT NOT NULL,public INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS records(member TEXT NOT NULL,key TEXT NOT NULL,date TEXT NOT NULL,tool TEXT NOT NULL,model TEXT NOT NULL,input TEXT,output TEXT,cacheRead TEXT,cacheWrite TEXT,PRIMARY KEY(member,key));`);
db.exec(`CREATE TABLE IF NOT EXISTS enrollments(ticket TEXT PRIMARY KEY, member TEXT NOT NULL, expires INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS device_tokens(secret TEXT PRIMARY KEY, member TEXT NOT NULL);`);
const auth=githubAuth(db,locate);
const registry=devices(db);
const hash = x=>createHash('sha256').update(x).digest('hex');
const send=(res,status,data)=>{res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store'});res.end(JSON.stringify(data));};
const fail=(message,status=400)=>Object.assign(new Error(message),{status});
export function normalize(records){
 if(!Array.isArray(records)||!records.length||records.length>50000) throw fail('请提供 1–50000 条 records 记录');
 return records.map((r,i)=>{
  if(r.kind!=='delta') throw fail(`第 ${i+1} 条不是 delta 增量记录，累计记录不能直接相加，请用采集器 export 导出`);
  if(typeof r.recordKey!=='string'||!r.recordKey||r.recordKey.length>256||typeof r.tool!=='string'||!r.tool||r.tool.length>128||!Number.isFinite(Date.parse(r.occurredAt))) throw fail(`第 ${i+1} 条缺少有效 recordKey、tool 或 occurredAt`);
  if(Date.parse(r.occurredAt)>Date.now()+5*60000||Date.parse(r.occurredAt)<Date.UTC(2000,0,1))throw fail(`第 ${i+1} 条时间超出有效范围`);
  if(r.model!=null&&(typeof r.model!=='string'||!r.model.trim()||Buffer.byteLength(r.model)>128||/[\x00-\x1f\x7f]/.test(r.model))) throw fail('model 不合法');
  const out={key:r.recordKey,date:new Date(r.occurredAt).toISOString(),tool:r.tool,model:r.model||'unknown'};
  for(const k of ['input','output','cacheRead','cacheWrite']) {const v=r[k];if(v!=null&&(typeof v!=='string'||!/^(0|[1-9][0-9]*)$/.test(v)||BigInt(v)>18446744073709551615n))throw fail(`第 ${i+1} 条 ${k} 必须是 uint64 十进制字符串或 null`);out[k]=v??null;}
  return out;
 });
}
async function body(req){let chunks=[],size=0;for await(const c of req){size+=c.length;if(size>12*1024*1024)throw fail('文件超过 12 MB',413);chunks.push(c);}try{return JSON.parse(Buffer.concat(chunks));}catch{throw fail('JSON 格式不正确');}}
function member(req,owner=false){const logged=auth.session(req);if(logged){if(!['GET','HEAD'].includes(req.method)&&req.headers['x-csrf-token']!==logged.csrf)throw fail('页面登录已过期，请刷新后重试',403);return logged;}const t=req.headers.authorization?.replace(/^Bearer /,'');if(!t)throw fail('请输入参与密钥',401);let m=db.prepare('SELECT * FROM members WHERE secret=?').get(hash(t));if(!m&&!owner){const d=registry.current(t);if(d){m=db.prepare('SELECT * FROM members WHERE id=?').get(d.member);if(m)m.deviceId=d.id;}}if(!m)throw fail('参与密钥无效或设备已撤销',401);return m;}
const server=http.createServer(async(req,res)=>{try{
 const u=new URL(req.url,'http://localhost');
 if(process.env.PUBLIC_ORIGIN&&req.headers.host!==new URL(process.env.PUBLIC_ORIGIN).host&&!['127.0.0.1:8081','localhost:8081'].includes(req.headers.host)){if(req.method==='GET'||req.method==='HEAD'){res.writeHead(308,{Location:process.env.PUBLIC_ORIGIN+u.pathname+u.search,'Cache-Control':'no-store'});res.end();return;}throw fail('请使用正式 HTTPS 域名访问',426);}
 if(!['GET','HEAD'].includes(req.method)&&req.headers.origin&&req.headers.origin!==`http://${req.headers.host}`&&req.headers.origin!==`https://${req.headers.host}`)throw fail('请求来源不允许',403);
 if(await auth.handle(req,res,u))return;
 if(u.pathname==='/api/profile'&&req.method==='PATCH'){const m=member(req,true);if(!auth.session(req))throw fail('请先通过 GitHub 登录',401);const b=await body(req);const name=String(b.name||'').trim();if(!name||name.length>32)throw fail('昵称需为 1–32 个字符');db.prepare('UPDATE members SET name=?,public=? WHERE id=?').run(name,typeof b.public==='boolean'?(b.public?1:0):m.public,m.id);return send(res,200,{ok:true});}
 if(u.pathname==='/api/health'&&req.method==='GET')return send(res,200,{ok:true,service:'DH TokenRank'});
 if(u.pathname==='/api/location'&&req.method==='GET')return send(res,200,await locate(req));
 if(u.pathname==='/api/enrollment'&&req.method==='POST'){const m=member(req,true);db.prepare('DELETE FROM enrollments WHERE member=? OR expires<?').run(m.id,Date.now());const ticket=randomBytes(32).toString('hex'),expires=Date.now()+15*60000;db.prepare('INSERT INTO enrollments VALUES(?,?,?)').run(hash(ticket),m.id,expires);return send(res,201,{ticket,expires,name:m.name});}
 if(u.pathname==='/api/enroll'&&req.method==='POST'){const b=await body(req);if(typeof b.ticket!=='string')throw fail('接入码无效',401);const entry=db.prepare('SELECT * FROM enrollments WHERE ticket=? AND expires>?').get(hash(b.ticket),Date.now());if(!entry)throw fail('接入码已使用或过期，请重新生成',401);return send(res,200,registry.enroll(entry,b));}
 if(u.pathname==='/api/members'&&req.method==='POST'){if(process.env.ALLOW_LEGACY_REGISTRATION!=='1')throw fail('请通过 GitHub 登录后参与',401);
  const b=await body(req);const name=String(b.name||'').trim(),city=(await locate(req)).city;if(!name||name.length>32||city.length>40)throw fail('昵称需为 1–32 个字符，城市最多 40 个字符');
  const id=randomBytes(12).toString('hex'),token=randomBytes(32).toString('hex');db.prepare('INSERT INTO members(id,name,city,secret,public) VALUES(?,?,?,?,?)').run(id,name,city,hash(token),b.public===true?1:0);return send(res,201,{id,token,city});
 }
 if(u.pathname==='/api/import'&&req.method==='POST'){
  const m=member(req),b=await body(req),rows=normalize(Array.isArray(b)?b:b.records);let added=0;
  return send(res,200,registry.ingest(m,rows));
 }
 if(u.pathname==='/api/me'&&req.method==='GET'){const m=member(req);return send(res,200,{name:m.name,city:m.city,public:!!m.public,memberId:m.id,deviceId:m.deviceId||null});}
 if(u.pathname==='/api/me'&&req.method==='PATCH'){const m=member(req,true),b=await body(req);if(typeof b.public!=='boolean')throw fail('public 必须为布尔值');db.prepare('UPDATE members SET public=? WHERE id=?').run(b.public?1:0,m.id);return send(res,200,{ok:true});}
 if(u.pathname==='/api/me'&&req.method==='DELETE'){const m=member(req,true);db.exec('BEGIN');try{db.prepare('DELETE FROM sessions WHERE member=?').run(m.id);db.prepare('DELETE FROM enrollments WHERE member=?').run(m.id);db.prepare('DELETE FROM device_token_history WHERE member=?').run(m.id);db.prepare('DELETE FROM device_tokens WHERE member=?').run(m.id);db.prepare('DELETE FROM records WHERE member=?').run(m.id);db.prepare('DELETE FROM members WHERE id=?').run(m.id);db.exec('COMMIT');}catch(e){db.exec('ROLLBACK');throw e;}return send(res,200,{ok:true});}
 if(u.pathname==='/api/device-status'&&req.method==='POST'){const m=member(req);if(!m.deviceId)throw fail('此接口仅用于设备同步状态',403);const b=await body(req);if(typeof b.ok!=='boolean')throw fail('状态无效');const failures=Array.isArray(b.failedSources)?b.failedSources.filter(x=>typeof x==='string'&&/^[a-z0-9-]{1,64}$/.test(x)).slice(0,20).join(','):'';db.prepare("UPDATE device_tokens SET lastSync=CASE WHEN ? THEN ? ELSE lastSync END,lastError=? WHERE id=?").run(b.ok?1:0,Date.now(),b.ok?'':failures||'sync-failed',m.deviceId);return send(res,200,{ok:true});}
 if(u.pathname==='/api/devices' &&req.method==='GET'){const m=member(req,true);return send(res,200,{devices:registry.list(m.id),unattributedRecords:db.prepare("SELECT COUNT(*) AS n FROM records WHERE member=? AND device IN ('','manual')").get(m.id).n});}
 if(/^\/api\/devices\/[a-f0-9]{24}\/revoke$/.test(u.pathname)&&req.method==='POST'){const m=member(req,true);const id=u.pathname.split('/')[3];const result=registry.revoke(m.id,id);if(!result.changes)throw fail('设备不存在',404);return send(res,200,{ok:true});}
 if(u.pathname==='/api/history' &&req.method==='GET'){
  const m=member(req),after=u.searchParams.get('after')||'';if(after.length>256)throw fail('分页参数无效');
  const rows=db.prepare('SELECT key AS recordKey,date AS occurredAt,tool,model,input,output,cacheRead,cacheWrite FROM records WHERE member=? AND key>? ORDER BY key LIMIT 1000').all(m.id,after);
  return send(res,200,{records:rows,next:rows.length===1000?rows.at(-1).recordKey:null});
 }
 if(u.pathname==='/api/records'&&req.method==='GET'){
  const rows=db.prepare(`SELECT m.id,m.name,m.city,m.avatar,m.github_login AS githubLogin,r.date,r.tool,r.model,r.input,r.output,r.cacheRead,r.cacheWrite FROM records r JOIN members m ON m.id=r.member WHERE m.public=1 AND r.flag='' ORDER BY r.date DESC`).all();return send(res,200,{records:rows});
 }
 if(u.pathname.startsWith('/api/'))throw fail('接口不存在',404);
 const files={'/':'index.html','/setup':'index.html','/app.js':'app.js','/style.css':'style.css','/agent-setup.md':'agent-setup.md','/public-security.png':'public-security.png'};const f=files[u.pathname];if(!f)throw fail('页面不存在',404);
 res.writeHead(200,{'Content-Type':f.endsWith('.png')?'image/png':f.endsWith('.md')?'text/markdown; charset=utf-8':f.endsWith('.css')?'text/css':f.endsWith('.js')?'text/javascript':'text/html; charset=utf-8','X-Content-Type-Options':'nosniff','Referrer-Policy':'no-referrer','Cache-Control':'no-store','Content-Security-Policy':"default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data: https://avatars.githubusercontent.com; base-uri 'none'; frame-ancestors 'none'"});const content=readFileSync(new URL(`./public/${f}`,import.meta.url));res.end(f==='index.html'?content.toString().replace('<body>','<body data-page="'+(u.pathname==='/setup'?'setup':'home')+'">'):content);
 }catch(e){send(res,e.status||500,{error:e.status?e.message:'服务器处理失败，请稍后重试'});}});
if(process.env.NODE_ENV!=='test')server.listen(Number(process.env.PORT||3080),process.env.HOST||'127.0.0.1',()=>console.log(`DH燃源榜 http://${process.env.HOST||'127.0.0.1'}:${process.env.PORT||3080}`));
export {server};
