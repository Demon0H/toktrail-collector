import {test,expect} from '@playwright/test';
test('desktop and mobile demo, filters and joining flow',async({page})=>{
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('/');await page.getByRole('button',{name:'看看演示'}).click();await expect(page.locator('#rows tr')).toHaveCount(10);
 await page.screenshot({path:'test-results/desktop.png',fullPage:true});
 await page.locator('#search').fill('Demon0H');await expect(page.locator('#rows tr')).toHaveCount(1);await page.locator('#cache').check();await expect(page.locator('.bar span').first()).not.toHaveCSS('width','0px');
 await page.locator('#search').fill('不存在的昵称');await expect(page.locator('#empty')).toBeVisible();await page.locator('#search').fill('');
 await page.getByRole('button',{name:'真实榜单',exact:true}).click();await page.locator('[name=name]').fill('UI-'+Date.now());await page.locator('[name=public]').check();await page.getByRole('button',{name:'创建参与身份'}).click();await expect(page.locator('#token')).not.toHaveValue('');
 await page.locator('#file').setInputFiles({name:'usage.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({records:[{recordKey:'ui-1',kind:'delta',occurredAt:new Date().toISOString(),tool:'Codex',model:'ui-model',input:'1000',output:'500',cacheRead:'20',cacheWrite:'0'}]}))});
 await page.getByRole('button',{name:'导入我的用量'}).click();await expect(page.locator('#feedback')).toContainText('新增 1 条');await expect(page.locator('#rows')).toContainText('UI-');
 await page.getByRole('button',{name:'撤回公开',exact:true}).click();await expect(page.locator('#feedback')).toContainText('已撤回公开');
 await page.setViewportSize({width:390,height:844});await page.getByRole('button',{name:'看看演示'}).click();await page.screenshot({path:'test-results/mobile.png',fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'删除我的全部数据'}).click();await expect(page.locator('#feedback')).toContainText('已删除');expect(errors).toEqual([]);
});
