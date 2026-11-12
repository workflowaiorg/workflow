import { request, type Schema } from './api'

export type HealthStatus = Schema<'HealthStatusResponse'>

export async function getHealth(): Promise<HealthStatus> {
  const answer = await request('get', '/health')
  return answer.data
}
