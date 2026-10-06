// Synthetic UI preview only. No requests to X or Stripe.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
const files = { "/": ["lite.html", "text/html; charset=utf-8"], "/lite.js": ["lite.js", "application/javascript"], "/appearance.js": ["appearance.js", "application/javascript"], "/favicon.svg": ["favicon.svg", "image/svg+xml"] };
const plans = [{ months: 3, amount: 30000, currency: "BDT" }, { months: 6, amount: 60000, currency: "BDT" }];
createServer(async (req,res) => {
  const path = new URL(req.url,"http://localhost").pathname;
  const json = (status,data) => { res.writeHead(status,{"Content-Type":"application/json"}); res.end(JSON.stringify(data)); };
  if (files[path]) {
    const [name,type] = files[path];
    let data = await readFile(`internal/site/assets/${name}`);
    if (name.endsWith("html")) data = Buffer.from(data.toString().replaceAll("__XGIFT_NONCE__","preview").replace("<title>","<title>模拟预览 · "));
    res.writeHead(200,{"Content-Type":type}); res.end(data); return;
  }
  if(path==="/api/security") return json(200,{turnstile_enabled:false});
  if(path==="/api/manual-link/plans") return json(200,{plans});
  let raw=""; for await(const chunk of req) raw+=chunk;
  let body={}; try { body=JSON.parse(raw||"{}"); } catch { return json(400,{}); }
  await new Promise(resolve=>setTimeout(resolve,700));
  if(path==="/api/check") return json(200,{eligible:body.username!=="ineligible",message:"模拟账号不允许接收赠送。"});
  if(path==="/api/manual-link") {
    if(body.username==="error") return json(502,{message:"模拟请求失败，请稍后重试。"});
    if(body.username==="queued") return json(202,{status:"queued",ticket:"demo",ahead:1,estimated_wait_seconds:900});
    return json(200,{...plans.find(p=>p.months===body.months),username:body.username,status:"created",checkout_url:"https://checkout.stripe.com/c/pay/cs_test_PREVIEWONLY",expires_at:Math.floor(Date.now()/1000)+900});
  }
  if(path==="/api/manual-link/queue/demo") return json(200,{status:"queued",ticket:"demo",ahead:1,estimated_wait_seconds:900});
  json(404,{message:"Preview route not found"});
}).listen(4173,"127.0.0.1",()=>console.log("Synthetic Lite preview: http://127.0.0.1:4173 (no real payment links)"));
