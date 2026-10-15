export class ApiError extends Error {
  status:number;
  uncertain:boolean;
  constructor(message:string, status:number, uncertain=false) { super(message); this.status=status;this.uncertain=uncertain; }
}
export class ApiClient {
  private controllers = new Set<AbortController>();
  private epoch=0;
  private fetcher:typeof fetch;
  constructor(fetcher:typeof fetch=fetch){this.fetcher=fetcher;}
  cancelAll() { this.epoch++; this.controllers.forEach(c=>c.abort()); this.controllers.clear(); }
  async request(path:string,key:string,method='GET',body?:Record<string,unknown>):Promise<unknown> {
    if(!path.startsWith('/') || path.startsWith('//') || /[\r\n]/.test(path)) throw new Error('Invalid API destination.');
    const controller = new AbortController(), epoch=this.epoch;
    this.controllers.add(controller);
    const timer=setTimeout(()=>controller.abort(),30000);
    const mutation=method!=='GET';
    try {
      const response=await this.fetcher(path,{method,headers:{apikey:key,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body),signal:controller.signal,cache:'no-store',credentials:'omit',redirect:'error'});
      if(epoch!==this.epoch) throw new ApiError('This console session was locked.',0);
      if(!response.headers.get('content-type')?.includes('application/json')) throw new ApiError(response.status===404?'This endpoint is unavailable here. The hosting proxy may allow connection controls only.':'The server returned an unexpected response. Check the deployment route.',response.status,mutation);
      // Stream with a bound so large contact/history/log responses cannot exhaust the tab.
      const reader=response.body?.getReader(); let bytes=0,text=''; const decoder=new TextDecoder();
      if(reader) { try { for(;;){const part=await reader.read(); if(part.done)break; bytes+=part.value.byteLength; if(bytes>2_000_000){await reader.cancel();throw new ApiError('Response is too large. Narrow the request or reduce its limit.',response.status);}text+=decoder.decode(part.value,{stream:true});}text+=decoder.decode();}finally{reader.releaseLock();} }
      if(epoch!==this.epoch) throw new ApiError('This console session was locked.',0);
      let value:unknown; try{value=text?JSON.parse(text):null;}catch{throw new ApiError('Invalid JSON response from server.',response.status,mutation);}
      if(!response.ok){
        const msg=response.status===401||response.status===403?'Access denied. Check the selected sender and API key.':response.status===404?'Endpoint not available on this deployment. Check the manager proxy allowlist.':response.status===429?'Too many requests. Wait before trying again.':`Service request failed (${response.status}).`;
        throw new ApiError(msg,response.status,mutation&&response.status>=500);
      }
      return value;
    } catch(error) {
      if(error instanceof ApiError)throw error;
      throw new ApiError(mutation?'Request interrupted. The action may already have happened. Check status before sending it again.':'Request interrupted or timed out. Check your connection.',0,mutation);
    } finally {clearTimeout(timer);this.controllers.delete(controller);}
  }
}
