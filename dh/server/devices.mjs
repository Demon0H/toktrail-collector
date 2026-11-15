import {randomBytes,createHash} from 'node:crypto';
const hash=s=>createHash('sha256').update(s).digest('hex');
const error=(message,status=400)=>Object.assign(Error(message),{status});
const clean=(v,max)=>String(v||'').replace(/[\x00-\x1f\x7f]/g,'').trim().slice(0,max);
export function devices(db){
 const column=(table,name,type)=>{if(!db.prepare(`PRAGMA table_info(${table})`).all().some(c=>c.name===name))db.exec(`ALTER TABLE ${table} ADD COLUMN ${name} ${type}`);};
 for(const [name,type]of [['id','TEXT'],['name',"TEXT NOT NULL DEFAULT '旧设备'"],['platform',"TEXT NOT NULL DEFAULT 'unknown'"],['created','INTEGER NOT NULL DEFAULT 0'],['lastSeen','INTEGER NOT NULL DEFAULT 0'],['revoked','INTEGER NOT NULL DEFAULT 0'],['lastSync','INTEGER NOT NULL DEFAULT 0'],['lastError',"TEXT NOT NULL DEFAULT ''"]])column('device_tokens',name,type);
 db.exec("UPDATE device_tokens SET id=lower(hex(randomblob(12))) WHERE id IS NULL; CREATE UNIQUE INDEX IF NOT EXISTS device_identity ON device_tokens(id);");
 column('records','device',"TEXT NOT NULL DEFAULT ''");column('records','flag',"TEXT NOT NULL DEFAULT ''");
 db.exec('CREATE TABLE IF NOT EXISTS device_token_history(secret TEXT PRIMARY KEY,member TEXT NOT NULL,device TEXT NOT NULL)');
 const lookup=token=>db.prepare('SELECT * FROM device_tokens WHERE secret=?').get(hash(token));
 const current=token=>{const d=lookup(token);return d&&!d.revoked?d:null;};
 function enroll(ticket,body){
  let prior=null;if(body.previousToken){prior=lookup(body.previousToken)||db.prepare('SELECT d.* FROM device_tokens d JOIN device_token_history h ON h.device=d.id WHERE h.secret=?').get(hash(body.previousToken));const legacy=db.prepare('SELECT id FROM members WHERE secret=?').get(hash(body.previousToken));if((prior?.member||legacy?.id)!==ticket.member)throw error('接入码与已有设备不是同一账号，请使用独立状态目录',409);}
  const id=prior?.id||randomBytes(12).toString('hex'),token=randomBytes(32).toString('hex');
  if(!prior&&db.prepare('SELECT COUNT(*) AS n FROM device_tokens WHERE member=? AND revoked=0').get(ticket.member).n>=32)throw error('活跃设备已达 32 台，请先撤销不再使用的设备',409);
  const platform=clean(body.platform,40)||prior?.platform||'unknown',name=clean(body.name,64)||prior?.name||platform+' · '+id.slice(0,6);
  db.exec('BEGIN');try{
   db.prepare('DELETE FROM enrollments WHERE ticket=?').run(ticket.ticket);
   if(prior){db.prepare('INSERT OR IGNORE INTO device_token_history VALUES(?,?,?)').run(prior.secret,prior.member,prior.id);db.prepare('UPDATE device_tokens SET secret=?,name=?,platform=?,revoked=0 WHERE id=?').run(hash(token),name,platform,id);}
   else db.prepare('INSERT INTO device_tokens(secret,member,id,name,platform,created,lastSeen,revoked) VALUES(?,?,?,?,?,?,0,0)').run(hash(token),ticket.member,id,name,platform,Date.now());
   db.exec('COMMIT');
  }catch(e){db.exec('ROLLBACK');throw e;}
  return {token,deviceId:id,memberId:ticket.member};
 }
 const windows=new Map();
 function ingest(member,rows){
  const key=member.id+':'+(member.deviceId||'manual'),now=Date.now();let window=windows.get(key);if(!window||now-window.start>=60000){if(windows.size>=5000)windows.clear();window={start:now,count:0};windows.set(key,window);}if(++window.count>120)throw error('上传频率过高，请一分钟后重试',429);
  const device=member.deviceId||'manual';let added=0,duplicates=0,quarantined=0;
  const existing=db.prepare('SELECT date,tool,model,input,output,cacheRead,cacheWrite,flag FROM records WHERE member=? AND key=?');
  const insert=db.prepare('INSERT INTO records(member,key,date,tool,model,input,output,cacheRead,cacheWrite,device,flag) VALUES(?,?,?,?,?,?,?,?,?,?,?)');
  db.exec('BEGIN');try{for(const r of rows){
   const old=existing.get(member.id,r.key);
   if(old){for(const k of ['date','tool','model','input','output','cacheRead','cacheWrite'])if(old[k]!==r[k])throw error('记录内容与已接收历史冲突，整批未写入；请勿修改已有记录',409);duplicates++;continue;}
   const total=['input','output','cacheRead','cacheWrite'].reduce((n,k)=>n+BigInt(r[k]||'0'),0n);const flag=total>1_000_000_000n?'token-count-outlier':'';
   insert.run(member.id,r.key,r.date,r.tool,r.model,r.input,r.output,r.cacheRead,r.cacheWrite,device,flag);added++;if(flag)quarantined++;
  }if(member.deviceId)db.prepare('UPDATE device_tokens SET lastSeen=? WHERE id=?').run(Date.now(),member.deviceId);db.exec('COMMIT');}catch(e){db.exec('ROLLBACK');throw e;}
  return {added,duplicates,quarantined};
 }
 const list=member=>db.prepare(`SELECT d.id,d.name,d.platform,d.created,d.lastSeen,d.lastSync,d.lastError,d.revoked,COUNT(r.key) AS records,COALESCE(SUM(r.flag!=''),0) AS quarantined FROM device_tokens d LEFT JOIN records r ON r.member=d.member AND r.device=d.id WHERE d.member=? GROUP BY d.id ORDER BY d.created`).all(member);
 const revoke=(member,id)=>db.prepare('UPDATE device_tokens SET revoked=1 WHERE id=? AND member=?').run(id,member);
 return {current,enroll,ingest,list,revoke};
}
