import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'./tests',testMatch:'*.spec.js',use:{channel:process.env.PLAYWRIGHT_CHANNEL || 'msedge',baseURL:'http://127.0.0.1:3081'},webServer:{command:'node server.mjs',url:'http://127.0.0.1:3081',env:{PORT:'3081',DATA_DIR:'data/ui-test'},reuseExistingServer:false}});
