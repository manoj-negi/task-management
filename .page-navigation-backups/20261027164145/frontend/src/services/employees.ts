const API_BASE = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')
export interface Employee { id:string; full_name:string; email:string; role:string; can_assign_tasks:boolean; is_active:boolean; created_at:string }
async function request<T>(token:string,path:string,init:RequestInit={}):Promise<T>{
 const response=await fetch(`${API_BASE}${path}`,{...init,headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`,...init.headers}})
 const data=await response.json().catch(()=>({})) as T & {error?:string}
 if(!response.ok) throw new Error(data.error || 'Request failed.')
 return data
}
export const listEmployees=(token:string)=>request<{employees:Employee[]}>(token,'/api/admin/employees')
export const createEmployee=(token:string,payload:{full_name:string;email:string;password:string})=>request<{employee:Employee}>(token,'/api/admin/employees',{method:'POST',body:JSON.stringify(payload)})
export const setAssigner=(token:string,id:string,enabled:boolean)=>request<{ok:boolean}>(token,`/api/admin/employees/${id}/permissions`,{method:'PATCH',body:JSON.stringify({can_assign_tasks:enabled})})
export const setEmployeeActive=(token:string,id:string,enabled:boolean)=>request<{ok:boolean}>(token,`/api/admin/employees/${id}`,{method:'PATCH',body:JSON.stringify({is_active:enabled})})
export const resetEmployeePassword=(token:string,id:string)=>request<{temporary_password:string;message:string}>(token,`/api/admin/employees/${id}/password`,{method:'POST',body:'{}'})
