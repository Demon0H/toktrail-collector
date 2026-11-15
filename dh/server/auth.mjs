import {randomBytes,createHash} from 'node:crypto';
const hash=value=>createHash('sha256').update(value).digest('hex');
const random=()=>randomBytes(32).toString('base64url');
export const cookies=req=>Object.fromEntries((req.headers.cookie||'').split(';').map(v=>v.trim().split('=')).filter(p=>p.length===2));
export function githubAuth(db,locate){
 if(!db.prepare('PRAGMA table_info(members)').all().some(c=>c.name==='github_id'))db.exec('ALTER TABLE members ADD COLUMN github_id TEXT');
 for(const name of ['avatar','github_login'])if(!db.prepare('PRAGMA table_info(members)').all().some(c=>c.name===name))db.exec(`ALTER TABLE members ADD COLUMN ${name} TEXT NOT NULL DEFAULT ''`);
 db.exec("UPDATE members SET avatar='https://avatars.githubusercontent.com/u/'||github_id||'?v=4' WHERE github_id IS NOT NULL AND avatar=''");
 db.exec('CREATE UNIQUE INDEX IF NOT EXISTS members_github ON members(github_id); CREATE TABLE IF NOT EXISTS sessions(secret TEXT PRIMARY KEY,member TEXT NOT NULL,csrf TEXT NOT NULL,expires INTEGER NOT NULL);');
 const pending=new Map();
 const configured=()=>!!(process.env.GITHUB_CLIENT_ID&&process.env.GITHUB_CLIENT_SECRET);
 const session=req=>{const key=cookies(req).dh_session;return key?db.prepare('SELECT m.*,s.csrf FROM members m JOIN sessions s ON m.id=s.member WHERE s.secret=? AND s.expires>?').get(hash(key),Date.now()):undefined;};
 const cookie=(name,value,age,path='/')=>`${name}=${value}; Path=${path}; HttpOnly; Secure; SameSite=Lax; Max-Age=${age}`;
 const redirect=(res,where,headers={})=>{res.writeHead(302,{Location:where,'Cache-Control':'no-store',...headers});res.end();};
 const json=(res,status,data)=>{res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store'});res.end(JSON.stringify(data));};
 async function handle(req,res,u){
  if(u.pathname==='/api/session'&&req.method==='GET'){const m=session(req);json(res,200,{configured:configured(),user:m?{name:m.name,city:m.city,public:!!m.public,github:true,avatar:m.avatar,githubLogin:m.github_login}:null,csrf:m?.csrf||null});return true;}
  if(u.pathname==='/auth/github'&&req.method==='GET'){
   if(!configured()){res.writeHead(503,{'Content-Type':'text/plain; charset=utf-8','Cache-Control':'no-store'});res.end('GitHub 登录正在配置，请稍后再试。');return true;}
   for(const [key,v]of pending)if(v.expires<Date.now())pending.delete(key);if(pending.size>=1000){json(res,429,{error:'登录请求过多，请稍后重试'});return true;}
   const state=random(),verifier=random();pending.set(state,{verifier,expires:Date.now()+600000});
   const query=new URLSearchParams({client_id:process.env.GITHUB_CLIENT_ID,redirect_uri:'https://tokenrank.demonh.site/auth/github/callback',scope:'',state,code_challenge:createHash('sha256').update(verifier).digest('base64url'),code_challenge_method:'S256'});
   redirect(res,'https://github.com/login/oauth/authorize?'+query,{'Set-Cookie':cookie('dh_oauth',state,600,'/auth')});return true;
  }
  if(u.pathname==='/auth/github/callback'&&req.method==='GET'){
   const state=u.searchParams.get('state'),item=pending.get(state);pending.delete(state);
   if(!item||item.expires<Date.now()||cookies(req).dh_oauth!==state||!u.searchParams.get('code')){redirect(res,'/setup?login=failed');return true;}
   try{
    const response=await fetch('https://github.com/login/oauth/access_token',{method:'POST',headers:{Accept:'application/json','Content-Type':'application/json'},body:JSON.stringify({client_id:process.env.GITHUB_CLIENT_ID,client_secret:process.env.GITHUB_CLIENT_SECRET,code:u.searchParams.get('code'),code_verifier:item.verifier,redirect_uri:'https://tokenrank.demonh.site/auth/github/callback'}),signal:AbortSignal.timeout(15000)});
    const token=await response.json();if(!response.ok||typeof token.access_token!=='string')throw Error();
    const profileResponse=await fetch('https://api.github.com/user',{headers:{Authorization:'Bearer '+token.access_token,Accept:'application/vnd.github+json','User-Agent':'DH-TokenRank'},signal:AbortSignal.timeout(15000)});const profile=await profileResponse.json();
    if(!profileResponse.ok||!Number.isSafeInteger(profile.id)||typeof profile.login!=='string')throw Error();
    const gh=String(profile.id),city=(await locate(req)).city;let m=db.prepare('SELECT * FROM members WHERE github_id=?').get(gh);
    if(!m){const id=randomBytes(12).toString('hex');db.prepare('INSERT INTO members(id,name,city,secret,public,github_id) VALUES(?,?,?,?,1,?)').run(id,String(profile.name||profile.login).slice(0,32),city,hash(random()),gh);m=db.prepare('SELECT * FROM members WHERE id=?').get(id);}
    else db.prepare('UPDATE members SET city=?,public=1 WHERE id=?').run(city||m.city,m.id);
    const avatar=typeof profile.avatar_url==='string'&&profile.avatar_url.startsWith('https://avatars.githubusercontent.com/')?profile.avatar_url:'https://avatars.githubusercontent.com/u/'+gh+'?v=4';
    db.prepare('UPDATE members SET avatar=?,github_login=? WHERE id=?').run(avatar,profile.login,m.id);
    const key=random(),csrf=random();db.prepare('DELETE FROM sessions WHERE expires<?').run(Date.now());db.prepare('INSERT INTO sessions VALUES(?,?,?,?)').run(hash(key),m.id,csrf,Date.now()+30*86400000);
    redirect(res,'/setup',{'Set-Cookie':[cookie('dh_session',key,30*86400),cookie('dh_oauth','',0,'/auth')]});
   }catch{redirect(res,'/setup?login=failed');}return true;
  }
  if(u.pathname==='/api/logout'&&req.method==='POST'){const m=session(req);if(!m||req.headers['x-csrf-token']!==m.csrf){json(res,403,{error:'登录已过期，请刷新重试'});return true;}db.prepare('DELETE FROM sessions WHERE secret=?').run(hash(cookies(req).dh_session));res.setHeader('Set-Cookie',cookie('dh_session','',0));json(res,200,{ok:true});return true;}
  return false;
 }
 return {handle,session,configured};
}
