import {readFileSync,writeFileSync,renameSync} from 'node:fs';
import {join} from 'node:path';

export function validateManifest(value){
 const semver=/^dh-v\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/;
 if(!value||!semver.test(value.version)||!semver.test(value.minVersion))throw Error('Invalid release metadata');
 const prefix=`https://github.com/Demon0H/toktrail-collector/releases/download/${value.version}/`;
 const asset=(v,name)=>{if(!v||v.url!==prefix+name||!/^[a-f0-9]{64}$/.test(v.sha256)||!Number.isSafeInteger(v.bytes)||v.bytes<=0)throw Error('Invalid release asset');return {url:v.url,sha256:v.sha256,bytes:v.bytes};};
 const assets={},installers={};for(const os of ['darwin','windows','linux'])for(const arch of ['amd64','arm64']){const format=os==='windows'?'zip':'tar.gz',key=`${os}-${arch}`;assets[key]={...asset(value.assets?.[key],`dh-tokenrank-${key}.${format}`),format};}
 for(const name of ['install.ps1','install.sh'])installers[name]=asset(value.installers?.[name],name);
 return {version:value.version,minVersion:value.minVersion,assets,installers,notes:'https://github.com/Demon0H/toktrail-collector/releases/tag/'+value.version};
}
export function releaseMetadata(dir){
 const path=join(dir,'release-metadata.json');let cached=null,checkedAt=0,inflight=null;try{const saved=JSON.parse(readFileSync(path,'utf8'));cached=validateManifest(saved.manifest);checkedAt=Number(saved.checkedAt)||0;}catch{}
 return async()=>{
  if(cached&&Date.now()-checkedAt<15*60000)return {...cached,checkedAt,stale:false};
  if(!inflight)inflight=(async()=>{try{const response=await fetch('https://github.com/Demon0H/toktrail-collector/releases/latest/download/latest.json',{signal:AbortSignal.timeout(12000)});if(!response.ok)throw Error();cached=validateManifest(await response.json());checkedAt=Date.now();try{writeFileSync(path+'.tmp',JSON.stringify({manifest:cached,checkedAt}),{mode:0o600});renameSync(path+'.tmp',path);}catch{}return {...cached,checkedAt,stale:false};}catch{if(cached)return {...cached,checkedAt,stale:true};throw Object.assign(Error('版本清单暂时不可用，请稍后重试或使用 GitHub Release 手动下载'),{status:503});}finally{inflight=null;}})();
  return inflight;
 };
}
