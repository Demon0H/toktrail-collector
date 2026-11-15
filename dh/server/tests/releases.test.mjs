import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {buildManifest} from '../../build-release-manifest.mjs';
import {validateManifest,releaseMetadata} from '../releases.mjs';

test('release manifest pins all six archives and installers with real hashes',()=>{
 const dir=mkdtempSync(join(tmpdir(),'dh-manifest-'));
 try{
  for(const os of ['darwin','windows','linux'])for(const arch of ['amd64','arm64'])writeFileSync(join(dir,`dh-tokenrank-${os}-${arch}.${os==='windows'?'zip':'tar.gz'}`),`${os}/${arch}`);
  for(const name of ['install.ps1','install.sh'])writeFileSync(join(dir,name),name);
  const manifest=buildManifest('dh-v1.1.2',dir),valid=validateManifest(manifest);
  assert.equal(Object.keys(valid.assets).length,6);
  assert.equal(valid.assets['windows-amd64'].sha256,createHash('sha256').update('windows/amd64').digest('hex'));
  assert.equal(readFileSync(join(dir,'SHA256SUMS'),'utf8').trim().split('\n').length,8);
  const bad=structuredClone(manifest);bad.assets['windows-amd64'].url='https://evil.invalid/binary.zip';assert.throws(()=>validateManifest(bad));
  bad.assets['windows-amd64']=manifest.assets['windows-amd64'];bad.assets['linux-arm64'].sha256='fake';assert.throws(()=>validateManifest(bad));
 }finally{rmSync(dir,{recursive:true,force:true});}
});

test('release lookup caches verified metadata and falls back only to verified stale data',async()=>{
 const dir=mkdtempSync(join(tmpdir(),'dh-release-cache-')),oldFetch=globalThis.fetch;
 try{
  for(const os of ['darwin','windows','linux'])for(const arch of ['amd64','arm64'])writeFileSync(join(dir,`dh-tokenrank-${os}-${arch}.${os==='windows'?'zip':'tar.gz'}`),'test');
  for(const name of ['install.ps1','install.sh'])writeFileSync(join(dir,name),'test');
  const manifest=buildManifest('dh-v1.1.2',dir);let calls=0;
  globalThis.fetch=async()=>{calls++;return {ok:true,json:async()=>manifest};};
  const get=releaseMetadata(dir);await Promise.all([get(),get(),get()]);assert.equal(calls,1);assert.equal((await get()).stale,false);assert.equal(calls,1);
  writeFileSync(join(dir,'release-metadata.json'),JSON.stringify({manifest,checkedAt:1}));globalThis.fetch=async()=>{throw Error('offline')};
  assert.equal((await releaseMetadata(dir)()).stale,true);
  rmSync(join(dir,'release-metadata.json'));await assert.rejects(releaseMetadata(dir)(),{status:503});
 }finally{globalThis.fetch=oldFetch;rmSync(dir,{recursive:true,force:true});}
});
