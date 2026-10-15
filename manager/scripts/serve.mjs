// Loopback-only source preview. No service credentials are stored by this server.
import http from 'node:http';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import {operations} from '../dist/assets/console/catalog.js';

const preview=process.argv.includes('--preview');
const allowActions=process.argv.includes('--allow-actions');
const port=Number(process.env.MANAGER_PORT||8092);
const upstream=new URL(process.env.MANAGER_API_ORIGIN||'http://127.0.0.1:8091');
if(!Number.isInteger(port)||port<1024||port>65535)throw Error('Use a valid nonprivileged preview port.');
if(upstream.origin!=='http://127.0.0.1:8091'||upstream.pathname!=='/'||upstream.search||upstream.username||upstream.password)throw Error('The development proxy must use the private local 8091 tunnel.');
const dist=fileURLToPath(new URL('../dist/',import.meta.url));
const fixtures=[
 {id:'00000000-0000-4000-8000-000000000001',name:'Demo · NextDeal OTP',token:'fixture-sender-key',connected:true,jid:'919999990330@s.whatsapp.net'},
 {id:'00000000-0000-4000-8000-000000000002',name:'Demo · Support',token:'fixture-support-key',connected:false,jid:''},
];
const matches=(op,method,p)=>op.method===method&&new RegExp('^'+op.path.split('/').map(s=>s.startsWith(':')?'[^/]+':s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')).join('/')+'$').test(p);
function json(res,status,body){res.writeHead(status,{'content-type':'application/json','cache-control':'no-store'});res.end(JSON.stringify(body));}
const server=http.createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://127.0.0.1:'+port),method=req.method||'GET';
  if(req.headers.host!==`127.0.0.1:${port}`&&req.headers.host!==`localhost:${port}`){json(res,403,{error:'Invalid local origin'});return;}
  res.setHeader('X-Content-Type-Options','nosniff');res.setHeader('Referrer-Policy','no-referrer');
  res.setHeader('Content-Security-Policy',"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'");
  if(method==='GET'&&(url.pathname==='/manager'||url.pathname.startsWith('/manager/')||url.pathname==='/')){
   let html=await readFile(path.join(dist,'index.html'),'utf8');
   if(preview)html=html.replace('</head>','<meta name="console-preview" content="true"></head>');
   if(!preview&&!allowActions)html=html.replace('</head>','<meta name="console-readonly" content="true"></head>');
   res.writeHead(200,{'content-type':'text/html','cache-control':'no-store'});res.end(html);return;
  }
  if(method==='GET'&&url.pathname.startsWith('/assets/console/')){
   const file=path.resolve(dist,'.'+decodeURIComponent(url.pathname));
   if(!file.startsWith(path.join(dist,'assets/console')+path.sep)){json(res,404,{error:'Not found'});return;}
   const data=await readFile(file);res.writeHead(200,{'content-type':file.endsWith('.css')?'text/css':'text/javascript','cache-control':'no-store'});res.end(data);return;
  }
  const op=operations.find(o=>matches(o,method,url.pathname));
  if(!op){json(res,404,{error:'Route is not in the manager operation catalog'});return;}
  if(op.blocked){json(res,409,{error:op.blocked});return;}
  if(!req.headers.apikey){json(res,401,{error:'API key required'});return;}
  if(preview){
   if(op.effect!=='read'){json(res,501,{error:'Preview mode does not perform actions'});return;}
   if(url.pathname==='/instance/all'){if(req.headers.apikey!=='preview-key'){json(res,401,{error:'Use preview-key for fictional data'});return;}json(res,200,{data:fixtures});return;}
   if(url.pathname==='/user/contacts'){json(res,200,{data:Array.from({length:42},(_,i)=>({Jid:`91999999${String(i).padStart(4,'0')}@s.whatsapp.net`,FullName:`Sample contact ${i+1}`,BusinessName:i%3===0?'Example company':''}))});return;}
   if(url.pathname==='/group/list'){json(res,200,{data:[{JID:'120000000000000@g.us',GroupName:'Demo · Operations'},{JID:'120000000000001@g.us',GroupName:'Demo · Support'}]});return;}
   if(url.pathname==='/label/list'){json(res,200,{data:[{label_id:'1',label_name:'New enquiry',label_color:'green'},{label_id:'2',label_name:'Follow up',label_color:'blue'}]});return;}
   if(url.pathname.includes('advanced-settings')){json(res,200,{alwaysOnline:false,rejectCall:false,msgRejectCall:'',readMessages:false,ignoreGroups:false,ignoreStatus:false});return;}
   if(url.pathname.startsWith('/instance/info/')){json(res,200,{data:fixtures[0]});return;}
   json(res,200,{message:'Synthetic preview response',data:{connected:true,fixture:true}});return;
  }
  if(op.effect!=='read'&&!allowActions){json(res,403,{error:'Local preview is read-only. Restart with --allow-actions to enable reviewed operator actions.'});return;}
  if(req.headers.origin&&req.headers.origin!==`http://${req.headers.host}`){json(res,403,{error:'Cross-origin action refused'});return;}
  let size=0;const parts=[];for await(const part of req){size+=part.length;if(size>1_000_000){json(res,413,{error:'Request too large'});return;}parts.push(part);}
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),28000);
  try{
   const response=await fetch(new URL(url.pathname+url.search,upstream),{method,headers:{'content-type':'application/json',apikey:String(req.headers.apikey)},body:['GET','HEAD'].includes(method)?undefined:Buffer.concat(parts),signal:controller.signal,redirect:'error'});
   const chunks=[];let count=0;for await(const chunk of response.body||[]){count+=chunk.byteLength;if(count>2_000_000)throw Error('Response too large');chunks.push(Buffer.from(chunk));}
   res.writeHead(response.status,{'content-type':response.headers.get('content-type')||'application/json','cache-control':'no-store'});res.end(Buffer.concat(chunks));
  }finally{clearTimeout(timer);}
 }catch{if(!res.headersSent)json(res,502,{error:'Local preview could not reach the private gateway. Check the SSH tunnel.'});else res.end();}
});
server.listen(port,'127.0.0.1',()=>console.log(`Manager ${preview?'SYNTHETIC PREVIEW':allowActions?'private gateway proxy (actions enabled)':'read-only private gateway proxy'}: http://127.0.0.1:${port}/manager · ${preview?'Key: preview-key':'Use your server manager key'}`));
