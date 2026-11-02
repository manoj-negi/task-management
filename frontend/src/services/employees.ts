const API_BASE = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')
export interface Employee { id:string; full_name:string; email:string; role:string; can_assign_tasks:boolean; is_active:boolean; created_at:string }
async function request<T>(path:string,init:RequestInit={}):Promise<T>{
const response = await fetch(`${API_BASE}${path}`, {
  ...init,
  credentials: 'include',
  headers: {
    ...init.headers,
    'Content-Type': 'application/json',
  },
})
 const data=await response.json().catch(()=>({})) as T & {error?:string}
 if(!response.ok) throw new Error(data.error || 'Request failed.')
 return data
}
export const listEmployees=()=>request<{employees:Employee[]}>('/api/admin/employees')
export const createEmployee=(payload:{full_name:string;email:string;password:string})=>request<{employee:Employee}>('/api/admin/employees',{method:'POST',body:JSON.stringify(payload)})
export const setAssigner=(id:string,enabled:boolean)=>request<{ok:boolean}>(`/api/admin/employees/${id}/permissions`,{method:'POST',body:JSON.stringify({can_assign_tasks:enabled})})
export const setEmployeeActive=(id:string,enabled:boolean)=>request<{ok:boolean}>(`/api/admin/employees/${id}`,{method:'POST',body:JSON.stringify({is_active:enabled})})
export const resetEmployeePassword=(id:string)=>request<{temporary_password:string;message:string}>(`/api/admin/employees/${id}/password`,{method:'POST',body:'{}'})
