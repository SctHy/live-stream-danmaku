import { api } from './business'

interface Taxonomy {
  categories: { code: string; name: string }[]
  strategies: { name: string; label: string }[]
}
let request: Promise<Taxonomy> | undefined
export function taxonomy() {
  return request ??= api<Taxonomy>('/api/plaza/strategies').catch(error => {
    request = undefined
    throw error
  })
}
export function categoryOptions(data: Taxonomy) {
  return data.categories.map(c => new Option(c.name, c.code))
}
export function roomFields(fields: Record<string, string>) {
  const tags = [...new Set(fields.tags.split(/[,，]/).map(t => t.trim()).filter(Boolean))]
  if (tags.length > 5 || tags.some(t => [...t].length > 16)) throw Error('最多 5 个标签，每个最多 16 个字')
  return { ...fields, tags }
}
