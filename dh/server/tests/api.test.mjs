import {test,after} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
const dir=mkdtempSync(join(tmpdir(),'dh-test-'));process.env.DATA_DIR=dir;process.env.NODE_ENV='test';
const {server,normalize}=await import('../server.mjs');
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const base=`http://127.0.0.1:${server.address().port}`;
async function request(path,method='GET',body,token=''){const res=await fetch(base+path,{method,headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`},body:body===undefined?undefined:JSON.stringify(body)});return {status:res.status,data:await res.json()};}
const record={recordKey:'safe-key',kind:'delta',occurredAt:new Date().toISOString(),tool:'Codex',model:'test-model',input:'9007199254740993',output:'25',cacheRead:null,cacheWrite:'0',prompt:'must never be stored'};
test('community lifecycle, opt-in, deduplication, ownership and privacy',async()=>{
 const a=await request('/api/members','POST',{name:'DH 测试',city:'杭州'});assert.equal(a.status,201);const token=a.data.token;
 assert.equal((await request('/api/import','POST',{records:[record]},'wrong')).status,401);
 assert.equal((await request('/api/import','POST',{records:[record]},token)).data.added,1);
 assert.equal((await request('/api/import','POST',{records:[record]},token)).data.duplicates,1);
 assert.deepEqual((await request('/api/records')).data.records,[]);
 await request('/api/me','PATCH',{public:true},token);const rows=(await request('/api/records')).data.records;assert.equal(rows.length,1);assert.equal(rows[0].input,'9007199254740993');assert.equal(rows[0].cacheRead,null);assert.equal(rows[0].prompt,undefined);assert.equal(rows[0].secret,undefined);
 const bad=await request('/api/import','POST',{records:[{...record,recordKey:'second'},{...record,input:'-1'}]},token);assert.equal(bad.status,400);assert.equal((await request('/api/records')).data.records.length,1);
 await request('/api/me','PATCH',{public:false},token);assert.equal((await request('/api/records')).data.records.length,0);
 await request('/api/me','DELETE',undefined,token);assert.equal((await request('/api/me','GET',undefined,token)).status,401);
});
test('reject cumulative counts, invalid dates and overflowing counters',()=>{for(const r of [{...record,kind:'cumulative'},{...record,occurredAt:'bad'},{...record,output:'18446744073709551616'},{...record,input:7}])assert.throws(()=>normalize([r]));assert.equal(normalize([record])[0].cacheRead,null);});
after(async()=>{await new Promise(resolve=>server.close(resolve));try{rmSync(dir,{recursive:true,force:true});}catch{/* SQLite may retain a Windows file handle until process exit. */}});
