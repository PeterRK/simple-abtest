export interface Application {
  id: number
  name: string
  access_token?: string
  version?: number
  description?: string
  experiment?: Experiment[]
}

export interface Experiment {
  id: number
  app_id?: number
  app_ver?: number
  status: number // 0: stopped, 1: active
  name: string
  version: number
  description?: string
  filter?: ExprNode[]
  layer?: Layer[]
}

export interface Layer {
  id: number
  exp_id?: number
  exp_ver?: number
  name: string
  version: number
  description?: string
  segment?: Segment[]
}

export interface Segment {
  id: number
  lyr_id?: number
  lyr_ver?: number
  begin: number
  end: number
  version: number
  group?: Group[]
}

export interface Group {
  id: number
  seg_id?: number
  seg_ver?: number
  share: number
  name: string
  is_default: boolean
  version: number
  cfg_id: number
  cfg_stamp?: string
  description?: string
  force_hit?: string[]
  config?: string
}

export interface Config {
  id: number
  stamp?: string
}

export interface ExprNode {
  op: number
  dtype?: number
  key?: string
  s?: string
  i?: number
  f?: number
  ss?: string[]
  child?: number[]
}
