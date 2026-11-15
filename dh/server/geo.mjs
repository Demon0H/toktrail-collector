import {isIP} from 'node:net';
const cache=new Map();
export function visitorIP(req){
 let ip=(req.socket.remoteAddress||'').replace(/^::ffff:/,'');
 // Only local TLS proxies and explicitly configured proxy addresses may set XFF.
 const trusted=['127.0.0.1','::1',...(process.env.TRUSTED_PROXIES||'').split(',')];
 if(trusted.includes(ip)&&req.headers['x-forwarded-for'])ip=req.headers['x-forwarded-for'].split(',').at(-1).trim().replace(/^::ffff:/,'');
 return isIP(ip)?ip:'';
}
export function publicIP(ip){
 if(!isIP(ip))return false;
 if(isIP(ip)===6)return !/^(::|fc|fd|fe[89ab]|ff)/i.test(ip);
 const [a,b]=ip.split('.').map(Number);return !(a===0||a===10||a===127||a>=224||(a===169&&b===254)||(a===172&&b>=16&&b<=31)||(a===192&&b===168)||(a===100&&b>=64&&b<=127));
}
export async function locate(req){
 const ip=visitorIP(req);if(!publicIP(ip))return {city:'',source:'unavailable'};
 const old=cache.get(ip);if(old&&old.expires>Date.now())return old.promise;
 const promise=(async()=>{try{const response=await fetch(`https://ipwho.is/${encodeURIComponent(ip)}?fields=success,city,region&lang=zh-CN`,{signal:AbortSignal.timeout(3500)});if(!response.ok)throw Error();const b=await response.json();return b.success&&typeof b.city==='string'&&b.city.trim()?{city:b.city.trim().slice(0,40),source:'ip'}:{city:'',source:'unavailable'};}catch{return {city:'',source:'unavailable'};}})();
 if(cache.size>=1000)cache.clear();cache.set(ip,{expires:Date.now()+15*60000,promise});return promise;
}
