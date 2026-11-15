import {test} from 'node:test';
import assert from 'node:assert/strict';
import {DatabaseSync} from 'node:sqlite';
import {githubAuth} from '../auth.mjs';
function res(){return {status:0,headers:{},writeHead(s,h){this.status=s;Object.assign(this.headers,h);},setHeader(k,v){this.headers[k]=v;},end(b){this.body=b;}};}
test('GitHub OAuth uses PKCE, binds state to cookie, creates public profile and revokes session',async()=>{
 const db=new DatabaseSync(':memory:');db.exec('CREATE TABLE members(id TEXT PRIMARY KEY,name TEXT,city TEXT,secret TEXT,public INTEGER)');const auth=githubAuth(db,async()=>({city:'杭州'}));
 const oldFetch=globalThis.fetch,oldID=process.env.GITHUB_CLIENT_ID,oldSecret=process.env.GITHUB_CLIENT_SECRET;process.env.GITHUB_CLIENT_ID='test-id';process.env.GITHUB_CLIENT_SECRET='test-secret';let verifier;
 globalThis.fetch=async(url,opt)=>{if(url==='https://github.com/login/oauth/access_token'){verifier=JSON.parse(opt.body).code_verifier;return {ok:true,json:async()=>({access_token:'test-access'})};}assert.equal(url,'https://api.github.com/user');assert.equal(opt.headers.Authorization,'Bearer test-access');return {ok:true,json:async()=>({id:123,login:'tester',name:'Test User'})};};
 try{
  const start=res();await auth.handle({method:'GET',headers:{}},start,new URL('https://tokenrank.demonh.site/auth/github'));const location=new URL(start.headers.Location);assert.equal(location.origin,'https://github.com');assert.equal(location.searchParams.get('scope'),'');assert.equal(location.searchParams.get('code_challenge_method'),'S256');const state=location.searchParams.get('state');
  const callback=res();await auth.handle({method:'GET',headers:{cookie:'dh_oauth='+state}},callback,new URL('https://tokenrank.demonh.site/auth/github/callback?code=test&state='+state));assert.equal(callback.status,302);assert.equal(callback.headers.Location,'/setup');assert.equal(verifier.length,43);assert.equal(db.prepare('SELECT public FROM members').get().public,1);
  const sessionCookie=callback.headers['Set-Cookie'][0].split(';')[0];assert.match(callback.headers['Set-Cookie'][0],/HttpOnly; Secure; SameSite=Lax/);const request={method:'GET',headers:{cookie:sessionCookie}};assert.equal(auth.session(request).name,'Test User');assert.equal(auth.session(request).avatar,'https://avatars.githubusercontent.com/u/123?v=4');
  const denied=res();await auth.handle({method:'POST',headers:{cookie:sessionCookie}},denied,new URL('https://tokenrank.demonh.site/api/logout'));assert.equal(denied.status,403);
  const loggedout=res();await auth.handle({method:'POST',headers:{cookie:sessionCookie,'x-csrf-token':auth.session(request).csrf}},loggedout,new URL('https://tokenrank.demonh.site/api/logout'));assert.equal(loggedout.status,200);assert.equal(auth.session(request),undefined);
  const replay=res();await auth.handle({method:'GET',headers:{cookie:'dh_oauth='+state}},replay,new URL('https://tokenrank.demonh.site/auth/github/callback?code=test&state='+state));assert.equal(replay.headers.Location,'/setup?login=failed');
 }finally{globalThis.fetch=oldFetch;if(oldID===undefined)delete process.env.GITHUB_CLIENT_ID;else process.env.GITHUB_CLIENT_ID=oldID;if(oldSecret===undefined)delete process.env.GITHUB_CLIENT_SECRET;else process.env.GITHUB_CLIENT_SECRET=oldSecret;db.close();}
});
