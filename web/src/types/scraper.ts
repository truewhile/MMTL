export type ScrapeTaskStatus = 'pending' | 'running' | 'done' | 'failed' | 'canceled'

export interface ScrapeTask {
  id: string
  media_id: string
  library_id: string
  library_name: string
  media_title: string
  media_path: string
  media_type: string
  provider: string
  matched_title: string
  matched_year: number
  poster_url: string
  backdrop_url: string
  status: ScrapeTaskStatus
  error: string
  retry_count: number
  episode_images: boolean
  refresh_matched: boolean
  created_at: string
  started_at?: string | null
  finished_at?: string | null
}

export interface ScrapeQueueCounts {
  pending: number
  running: number
  done: number
  failed: number
  canceled: number
}

export interface ScrapeQueueSnapshot {
  counts: ScrapeQueueCounts
  tasks: ScrapeTask[]
  total: number
  page: number
  page_size: number
}
