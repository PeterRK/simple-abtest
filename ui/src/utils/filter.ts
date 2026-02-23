import type { ExprNode } from '@/api/types'

export type { ExprNode }

export interface TreeNode {
  id: string // unique id for UI key
  op: number
  dtype?: number
  key?: string
  s?: string
  i?: number
  f?: number
  ss?: string[]
  children: TreeNode[]
}

let idCounter = 0
const generateId = () => `node_${Date.now()}_${idCounter++}`

export const flatToTree = (nodes: ExprNode[]): TreeNode | null => {
  if (!nodes || nodes.length === 0) return null

  const build = (index: number): TreeNode => {
    const node = nodes[index]
    if (!node) {
        // Should not happen if valid config
        return { id: generateId(), op: 0, children: [] }
    }
    const treeNode: TreeNode = {
      id: generateId(),
      op: node.op,
      dtype: node.dtype,
      key: node.key,
      s: node.s,
      i: node.i,
      f: node.f,
      ss: node.ss,
      children: []
    }
    if (node.child && node.child.length > 0) {
      treeNode.children = node.child.map(childIndex => build(childIndex))
    }
    return treeNode
  }

  return build(0)
}

export const treeToFlat = (root: TreeNode | null): ExprNode[] => {
  if (!root) return []

  const nodes: ExprNode[] = []
  
  const visit = (node: TreeNode): number => {
    const currentIndex = nodes.length
    // Placeholder
    nodes.push({} as ExprNode)
    
    const childIndices: number[] = []
    if (node.children) {
      for (const child of node.children) {
        childIndices.push(visit(child))
      }
    }

    nodes[currentIndex] = {
      op: node.op,
      dtype: node.dtype,
      key: node.key,
      s: node.s,
      i: node.i,
      f: node.f,
      ss: node.ss,
      child: childIndices.length > 0 ? childIndices : undefined
    }
    return currentIndex
  }

  visit(root)
  return nodes
}

export const OpTypes = {
  OpNull: 0,
  OpAnd: 1,
  OpOr: 2,
  OpNot: 3,
  OpIn: 4,
  OpNotIn: 5,
  OpEqual: 6,
  OpNotEqual: 7,
  OpLessThan: 8,
  OpLessOrEqual: 9,
  OpGreatThan: 10,
  OpGreatOrEqual: 11
}

export const DataTypes = {
  DtNull: 0,
  DtStr: 1,
  DtInt: 2,
  DtFloat: 3
}

export const OpOptions = [
  { label: 'AND', value: 1 },
  { label: 'OR', value: 2 },
  { label: 'NOT', value: 3 },
  { label: 'IN', value: 4 },
  { label: 'NOT IN', value: 5 },
  { label: '=', value: 6 },
  { label: '!=', value: 7 },
  { label: '<', value: 8 },
  { label: '<=', value: 9 },
  { label: '>', value: 10 },
  { label: '>=', value: 11 }
]

export const DTypeOptions = [
  { label: 'String', value: 1 },
  { label: 'Int', value: 2 },
  { label: 'Float', value: 3 }
]
