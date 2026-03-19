import axios from 'axios'
import type { Application, Experiment, Layer, Segment, Group, Config } from '@/types'
import type { ExprNode } from '@/types'
import { clearSession } from '@/auth'

const adminApi = axios.create({
  baseURL: '/api',
  withCredentials: true
})

const engineApi = axios.create({
  baseURL: '/engine'
})

adminApi.interceptors.response.use(
  (resp) => resp,
  (error) => {
    const status = error?.response?.status
    const method = (error?.config?.method || '').toLowerCase()
    const url = String(error?.config?.url || '')
    const isPasswordVerifyApi = (method === 'put' || method === 'delete') && /^\/user\/\d+$/.test(url)
    if (status === 401 && !isPasswordVerifyApi) {
      clearSession()
    }
    return Promise.reject(error)
  }
)

// User
export const registerUser = (data: { name: string; password: string; secret: string }) =>
  adminApi.post<{ uid: number }>('/user', data)
export const loginUser = (data: { name: string; password: string }) =>
  adminApi.post<{ uid: number }>('/user/login', data)
export const updateUserPassword = (uid: number, data: { password: string; new_password: string }) => adminApi.put(`/user/${uid}`, data)
export const deleteUser = (uid: number, data: { password: string }) => adminApi.delete(`/user/${uid}`, { data })

// Application
export const getApps = () => adminApi.get<Application[]>('/app')
export const getApp = (id: number) => adminApi.get<Application>(`/app/${id}`)
export const createApp = (data: { name: string; description?: string }) => adminApi.post<Application>('/app', data)
export const updateApp = (id: number, data: { name: string; description?: string; version: number }) => adminApi.put<Application>(`/app/${id}`, data)
export const deleteApp = (id: number, data: { version: number }) => adminApi.delete(`/app/${id}`, { data })
export const getAppPrivileges = (id: number) =>
  adminApi.get<{ name: string; privilege: number; grantor: string }[]>(`/app/${id}/privilege`)
export const grantAppPrivilege = (id: number, data: { name: string; privilege: number }) =>
  adminApi.post(`/app/${id}/privilege`, data)

// Experiment
export const createExp = (data: { app_id: number; app_ver: number; name: string; description?: string }) => adminApi.post<Experiment>('/exp', data)
export const getExp = (id: number) => adminApi.get<Experiment>(`/exp/${id}`)
export const updateExp = (id: number, data: { name: string; description?: string; version: number; filter?: ExprNode[] }) =>
  adminApi.put<Experiment>(`/exp/${id}`, data)
export const deleteExp = (id: number, data: { app_id: number; app_ver: number; version: number }) => adminApi.delete(`/exp/${id}`, { data })
export const shuffleExp = (id: number) => adminApi.post(`/exp/${id}/shuffle`)
export const switchExp = (id: number, data: { status: number; version: number }) => adminApi.put(`/exp/${id}/status`, data)

// Layer
export const createLayer = (data: { exp_id: number; exp_ver: number; name: string }) => adminApi.post<Layer>('/lyr', data)
export const getLayer = (id: number) => adminApi.get<Layer>(`/lyr/${id}`)
export const updateLayer = (id: number, data: { name: string; version: number }) => adminApi.put<Layer>(`/lyr/${id}`, data)
export const deleteLayer = (id: number, data: { exp_id: number; exp_ver: number; version: number }) => adminApi.delete(`/lyr/${id}`, { data })
export const rebalanceLayer = (id: number, data: { version: number; segment: { id: number; begin: number; end: number }[] }) => adminApi.post(`/lyr/${id}/rebalance`, data)

// Segment
export const createSegment = (data: { lyr_id: number; lyr_ver: number }) => adminApi.post<Segment>('/seg', data)
export const getSegment = (id: number) => adminApi.get<Segment>(`/seg/${id}`)
export const deleteSegment = (id: number, data: { lyr_id: number; lyr_ver: number; version: number }) => adminApi.delete(`/seg/${id}`, { data })
export const shuffleSegment = (id: number) => adminApi.post(`/seg/${id}/shuffle`)
export const rebalanceSegment = (id: number, data: { version: number; grp_id: number; share: number }) => adminApi.post(`/seg/${id}/rebalance`, data)

// Group
export const createGroup = (data: { seg_id: number; seg_ver: number; name: string }) => adminApi.post<Group>('/grp', data)
export const getGroup = (id: number) => adminApi.get<Group>(`/grp/${id}`)
export const updateGroup = (id: number, data: { name: string; version: number; cfg_id: number; force_hit?: string[] }) => adminApi.put<Group>(`/grp/${id}`, data)
export const deleteGroup = (id: number, data: { seg_id: number; seg_ver: number; version: number }) => adminApi.delete(`/grp/${id}`, { data })

// Config
export const getConfigs = (grpId: number, begin?: number) => adminApi.get<Config[]>(`/grp/${grpId}/cfg`, { params: { begin } })
export const createConfig = (grpId: number, content: string) => adminApi.post<{ id: number; stamp?: string }>(`/grp/${grpId}/cfg`, content)
export const getConfig = (grpId: number, cfgId: number) =>
  adminApi.get<string>(`/grp/${grpId}/cfg/${cfgId}`, { responseType: 'text' })

// Engine
export const verify = (data: { appid: number; key: string; context?: Record<string, string> }, accessToken: string) =>
  engineApi.post<{ config: Record<string, string>; tags: string[] }>('/', data, {
    headers: {
      ACCESS_TOKEN: accessToken
    }
  })
