import {createHash} from 'node:crypto';
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

export function buildManifest(version,dir){
 if(!/^dh-v\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version))throw Error('Invalid DH release tag');
 const base=`https://github.com/Demon0H/toktrail-collector/releases/download/${version}/`;
 const assets={},installers={},checksums=[];
 function file(name){const bytes=readFileSync(resolve(dir,name)),sha256=createHash('sha256').update(bytes).digest('hex');checksums.push(`${sha256}  ${name}`);return {url:base+name,sha256,bytes:bytes.length};}
 for(const os of ['darwin','windows','linux'])for(const arch of ['amd64','arm64']){const format=os==='windows'?'zip':'tar.gz';assets[`${os}-${arch}`]={...file(`dh-tokenrank-${os}-${arch}.${format}`),format};}
 for(const name of ['install.ps1','install.sh'])installers[name]=file(name);
 const manifest={version,minVersion:'dh-v1.1.1',generatedAt:new Date().toISOString(),assets,installers,notes:'https://github.com/Demon0H/toktrail-collector/releases/tag/'+version};
 writeFileSync(resolve(dir,'SHA256SUMS'),checksums.join('\n')+'\n');writeFileSync(resolve(dir,'latest.json'),JSON.stringify(manifest,null,2)+'\n');return manifest;
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href)buildManifest(process.argv[2],resolve(process.argv[3]||'.'));
