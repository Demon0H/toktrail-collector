import {test} from 'node:test';
import assert from 'node:assert/strict';
import {visitorIP,publicIP,locate} from '../geo.mjs';
test('only trusted proxies may supply visitor IP',()=>{assert.equal(visitorIP({socket:{remoteAddress:'8.8.8.8'},headers:{'x-forwarded-for':'1.1.1.1'}}),'8.8.8.8');assert.equal(visitorIP({socket:{remoteAddress:'127.0.0.1'},headers:{'x-forwarded-for':'8.8.4.4'}}),'8.8.4.4');assert.equal(visitorIP({socket:{remoteAddress:'::ffff:127.0.0.1'},headers:{'x-forwarded-for':'invalid'}}),'');});
test('private addresses do not leave server for geolocation',async()=>{for(const ip of ['127.0.0.1','10.0.0.1','192.168.1.1','172.20.0.1','::1','fc00::1','100.64.1.1'])assert.equal(publicIP(ip),false);assert.equal(publicIP('8.8.8.8'),true);assert.deepEqual(await locate({socket:{remoteAddress:'127.0.0.1'},headers:{}}),{city:'',source:'unavailable'});});
