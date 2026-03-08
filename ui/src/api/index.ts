import axios from 'axios'
import type { Application, Experiment, Layer, Segment, Group, Config } from '@/types'

const adminApi = axios.create({
  baseURL: '/api'
})

const engineApi = axios.create({
  baseURL: '/engine'
})

// Application
export const getApps = () => adminApi.get<Application[]>('/app')
export const getApp = (id: number) => adminApi.get<Application>(`/app/${id}`)
export const createApp = (data: { name: string; description?: string }) => adminApi.post<Application>('/app', data)
export const updateApp = (id: number, data: { name: string; description?: string; version: number }) => adminApi.put<Application>(`/app/${id}`, data)
export const deleteApp = (id: number, data: { version: number }) => adminApi.delete(`/app/${id}`, { data })

// Experiment
export const createExp = (data: { app_id: number; app_ver: number; name: string; description?: string }) => adminApi.post<Experiment>('/exp', data)
export const getExp = (id: number) => adminApi.get<Experiment>(`/exp/${id}`)
export const updateExp = (id: number, data: { name: string; description?: string; version: number; filter?: any[] }) => adminApi.put<Experiment>(`/exp/${id}`, data)
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
export const getConfig = (id: number) => adminApi.get<string>(`/cfg/${id}`, { responseType: 'text' })

// Engine
export const verify = (data: { appid: number; key: string; context?: Record<string, string> }) => engineApi.post<{ config: Record<string, string>; tags: string[] }>('/', data)

// Aliases for compatibility with existing components
export const createLyr = createLayer
export const updateLyr = updateLayer
export const deleteLyr = deleteLayer
export const rebalanceLyr = rebalanceLayer

export const createSeg = createSegment
export const deleteSeg = deleteSegment
export const shuffleSeg = shuffleSegment
export const rebalanceSeg = rebalanceSegment

export const createGrp = createGroup
export const updateGrp = updateGroup
export const deleteGrp = deleteGroup

export const getGrpCfg = getConfigs
export const createGrpCfg = createConfig
