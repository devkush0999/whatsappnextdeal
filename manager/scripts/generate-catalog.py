"""Generate the manager's operation inventory from mounted Go routes and Swagger DTOs.

Swagger is a field-discovery aid. Overrides below resolve known stale schemas;
runtime availability is not inferred from a route declaration.
"""
import argparse
import json
import re
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--repo', type=Path, default=Path(__file__).resolve().parents[2])
args = p.parse_args()
spec = json.loads((args.repo/'docs/swagger.json').read_text())
route_text = (args.repo/'pkg/routes/routes.go').read_text()
output = Path(__file__).resolve().parents[1]/'src/catalog.ts'

required = {
 '/send/text':['number','text'], '/send/link':['number','url','text'],
 '/send/media':['number','type','url'], '/send/poll':['number','question','options','maxAnswer'],
 '/send/sticker':['number','sticker'], '/send/location':['number','latitude','longitude'],
 '/send/contact':['number','vcard'], '/send/button':['number','title','description','footer','buttons'],
 '/send/list':['number','title','description','buttonText','sections'], '/send/carousel':['number','cards'],
 '/send/status/text':['text'], '/send/status/media':['type','url'],
 '/user/info':['number'], '/user/check':['number'], '/user/avatar':['number'],
 '/user/block':['number'], '/user/unblock':['number'], '/user/profileName':['name'],
 '/user/profileStatus':['status'], '/user/profilePicture':['image'],
 '/group/create':['groupName','participants'], '/group/join':['code'],
 '/group/name':['groupJid','name'], '/group/description':['groupJid','description'],
 '/group/info':['groupJid'], '/group/invitelink':['groupJid'], '/group/photo':['groupJid','image'],
 '/group/participant':['groupJid','participants','action'], '/group/leave':['groupJid'],
 '/group/settings':['groupJid','action'], '/community/create':['communityName'],
 '/community/add':['communityJid','groupJid'], '/community/remove':['communityJid','groupJid'],
 '/label/edit':['labelId','name'], '/label/chat':['jid','labelId'], '/label/message':['jid','labelId','messageId'],
 '/unlabel/chat':['jid','labelId'], '/unlabel/message':['jid','labelId','messageId'],
 '/newsletter/create':['name'], '/newsletter/info':['jid'], '/newsletter/link':['key'],
 '/newsletter/subscribe':['jid'], '/newsletter/messages':['jid','count'],
 '/message/status':['id'], '/message/react':['number','id','reaction'],
 '/message/edit':['chat','messageId','message'], '/message/delete':['chat','messageId'],
 '/message/markread':['number','id'], '/message/markplayed':['number','id'],
 '/message/presence':['number','state'], '/message/downloadmedia':['message'],
 '/chat/history-sync':['count','messageInfo'], '/call/reject':['callCreator','callId'],
 '/instance/create':['name','token'], '/instance/pair':['phone'],
 '/instance/forcereconnect/:instanceId':['number'], '/instance/proxy/:instanceId':['host','port'],
}
titles={
 '/send/text':'Text message','/send/link':'Link preview','/send/media':'Photo, video or document',
 '/send/poll':'Create a poll','/send/sticker':'Sticker','/send/location':'Share a location',
 '/send/contact':'Contact card','/send/button':'Reply buttons','/send/list':'List message',
 '/send/carousel':'Carousel','/send/status/text':'Text status','/send/status/media':'Photo or video status',
 '/user/contacts':'Contact directory','/user/info':'Contact information','/user/check':'Check WhatsApp availability',
 '/user/avatar':'Profile picture lookup','/user/blocklist':'Blocked contacts','/user/block':'Block a contact',
 '/user/unblock':'Unblock a contact','/user/profileName':'Update display name','/user/profileStatus':'Update about',
 '/user/profilePicture':'Update profile photo','/group/list':'Joined groups','/group/myall':'Group memberships',
 '/label/list':'All labels','/newsletter/list':'Subscribed channels','/message/status':'Message status',
 '/instance/all':'All instances','/instance/info/:instanceId':'Instance details',
 '/instance/logs/:instanceId':'Instance logs','/instance/forcereconnect/:instanceId':'Recover sender identity',
 '/instance/pair':'Link with phone code','/instance/qr':'Link with QR','/instance/status':'Connection status',
}
advanced={'delay','formatJid','forwardingScore','id','mentionAll','mentionedJid','quoted','instanceId','advancedSettings','proxy'}
read_posts={'/user/info','/user/check','/user/avatar','/group/info','/message/status','/message/downloadmedia','/newsletter/info','/newsletter/link','/newsletter/messages'}
blocked={
 '/chat/pin':'Backend route is marked not working. Tracked for recovery.',
 '/chat/unpin':'Backend route is marked not working. Tracked for recovery.',
 '/chat/archive':'Backend route is marked not working. Tracked for recovery.',
 '/chat/unarchive':'Backend route is marked not working. Tracked for recovery.',
 '/chat/mute':'Backend route is marked not working. Tracked for recovery.',
 '/chat/unmute':'Backend route is marked not working. Tracked for recovery.',
 '/group/myall':'Backend route is marked not working. Use Joined groups.',
 '/message/status':'Stored status is not yet scoped per sender. Restore ownership before enabling this lookup.',
}
def human(s):
 return re.sub(r'([a-z])([A-Z])',r'\1 \2',s).replace('_',' ').capitalize()
def schema_fields(schema, path):
 props=schema.get('properties',{})
 if path=='/user/profileName': props={'name':{'type':'string'}}
 if path=='/user/profileStatus': props={'status':{'type':'string'}}
 if path=='/send/status/media': props={k:{'type':'string'} for k in ['type','url','caption','id']}
 fields=[]
 for name,v in props.items():
  typ=v.get('type','string'); ref=v.get('$ref','')
  if ref and ref.split('.')[-1] not in ['JID','PrivacySetting','ParticipantChange']: typ='object'
  if name in ['groupJid','communityJid','callCreator','jid'] and typ!='array':typ='string'
  if typ=='array':typ='lines' if v.get('items',{}).get('type')=='string' else 'json-array'
  if typ=='object':typ='json-object'
  if name=='buttons' and path=='/send/button':typ='reply-buttons'
  if name=='sections' and path=='/send/list':typ='list-rows'
  if name=='vcard':typ='contact-card'
  f={'name':name,'label':human(name),'type':typ,'required':name in required.get(path,[]) or name in schema.get('required',[]),'advanced':name in advanced and name not in required.get(path,[])}
  if v.get('enum'):f['options']=v['enum']
  if name=='latitude':f.update(min=-90,max=90)
  if name=='longitude':f.update(min=-180,max=180)
  if name=='delay':f.update(min=0,max=60000)
  if name=='maxAnswer':f.update(min=1,max=12,default=1)
  if name=='count':f.update(min=1,max=100,default=20)
  if name=='before_id':f.update(min=0)
  if path in ['/send/media','/send/status/media'] and name=='type':f['options']=['image','video']+([] if '/status/' in path else ['document','audio'])
  if path=='/message/presence' and name=='state':f['options']=['composing','paused']
  if path=='/group/participant' and name=='action':f['options']=['add','remove','promote','demote']
  if path=='/group/settings' and name=='action':f['options']=['announcement','not_announcement','locked','unlocked']
  if path=='/instance/connect' and name in ['rabbitmqEnable','websocketEnable','natsEnable']:f['options']=['true','false']
  if path=='/user/privacy':f['required']=True
  if path=='/instance/create' and name=='token':f['advanced']=True
  fields.append(f)
 return sorted(fields,key=lambda f:(f['advanced'],0 if f['name']=='number' else 1))
items=[];group='';admin=False
for line in route_text.splitlines():
 m=re.search(r'Group\("([^"]+)"',line)
 if m:group=m[1];admin=False
 if 'AuthAdmin)' in line:admin=True
 if 'routes.Use(r.authMiddleware.Auth)' in line:admin=False
 m=re.search(r'routes\.(GET|POST|PUT|DELETE)\("([^"]+)"',line)
 if not m:continue
 method,tail=m.groups();path=group+tail;swagger_path=re.sub(r':(\w+)',r'{\1}',path)
 op=spec['paths'].get(swagger_path,{}).get(method.lower(),{})
 ref=next((v.get('schema',{}).get('$ref','') for v in op.get('parameters',[]) if v.get('in')=='body'),'')
 schema=spec['definitions'].get(ref.split('/')[-1],{})
 category=group[1:]
 if category=='unlabel':category='label'
 if category=='send':category='compose'
 if category=='user':category='profile' if '/privacy' in path or '/profile' in path else 'contacts'
 effect='read' if method=='GET' or path in read_posts else 'send' if category=='compose' else 'change'
 if method=='DELETE' or path in ['/group/leave','/message/delete'] or path.startswith('/unlabel/'):effect='destructive'
 if path=='/group/invitelink':effect='change'
 title=titles.get(path,op.get('summary') or human(tail.split('/')[-1]))
 if path=='/user/privacy':title='Privacy settings' if method=='GET' else 'Update privacy'
 if 'advanced-settings' in path:title='Sender behavior' if method=='GET' else 'Update sender behavior'
 if '/proxy/' in path:title='Configure proxy' if method=='POST' else 'Remove proxy'
 fields=schema_fields(schema,path)
 for name in re.findall(r':(\w+)',path):
  if name!='instanceId':fields.insert(0,{'name':name,'label':human(name),'type':'string','required':True,'in':'path'})
 if path.startswith('/instance/logs/'):
  fields += [{'name':'limit','label':'Maximum entries','type':'integer','required':False,'min':1,'max':100,'default':50,'in':'query'}, {'name':'level','label':'Log level','type':'string','required':False,'options':['INFO','WARN','ERROR','DEBUG'],'in':'query'}]
 item={'id':method+' '+path,'method':method,'path':path,'title':title,'category':category,'auth':'admin' if admin else 'instance','effect':effect,'fields':fields}
 if path in blocked:item['blocked']=blocked[path]
 if category in ['compose','community','newsletter','group']:item['note']='Availability depends on the linked account and WhatsApp client. Acceptance is not delivery confirmation.'
 if path=='/instance/connect':item['note']='Blank optional fields may reset existing event integrations. Load current configuration first.'
 if path=='/send/button':item['note']='This form creates up to three reply buttons. Mixing reply and CTA buttons is not supported.'
 if path=='/instance/forcereconnect/:instanceId':item['note']='Changes the stored sender identity. Use only when normal reconnect cannot recover the intended account.'
 items.append(item)
header='''// Generated by scripts/generate-catalog.py from mounted Go routes. Do not edit by hand.
export type FieldType = "string" | "integer" | "number" | "boolean" | "lines" | "json-array" | "json-object" | "reply-buttons" | "list-rows" | "contact-card";
export interface Field { name:string; label:string; type:FieldType; required?:boolean; advanced?:boolean; options?:Array<string|number>; min?:number; max?:number; default?:number|string|boolean; in?:"path"|"query"; }
export interface Operation { id:string; method:"GET"|"POST"|"PUT"|"DELETE"; path:string; title:string; category:string; auth:"admin"|"instance"; effect:"read"|"send"|"change"|"destructive"; fields:Field[]; blocked?:string; note?:string; }
'''
output.write_text(header+'export const operations: Operation[] = '+json.dumps(items,indent=2,ensure_ascii=False)+';\n')
print(f'Generated {len(items)} mounted operations; {sum(bool(x.get("blocked")) for x in items)} explicitly held for backend recovery.')
