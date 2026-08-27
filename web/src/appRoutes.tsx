/* eslint-disable react-refresh/only-export-components */
import { lazy, type ReactElement } from 'react'
import { Navigate } from 'react-router-dom'

const HomePage = lazy(() => import('./pages/HomePage').then((m) => ({ default: m.HomePage })))
const LibraryPage = lazy(() => import('./pages/LibraryPage').then((m) => ({ default: m.LibraryPage })))
const LibrariesPage = lazy(() => import('./pages/LibrariesPage').then((m) => ({ default: m.LibrariesPage })))
const FavouritesPage = lazy(() => import('./pages/FavouritesPage').then((m) => ({ default: m.FavouritesPage })))
const PlaylistsPage = lazy(() => import('./pages/PlaylistsPage').then((m) => ({ default: m.PlaylistsPage })))
const PlaylistDetailPage = lazy(() =>
  import('./pages/PlaylistDetailPage').then((m) => ({ default: m.PlaylistDetailPage })),
)
const MediaDetailPage = lazy(() => import('./pages/MediaDetailPage').then((m) => ({ default: m.MediaDetailPage })))
const PlayerPage = lazy(() => import('./pages/PlayerPage').then((m) => ({ default: m.PlayerPage })))
const AdminPage = lazy(() => import('./pages/AdminPage').then((m) => ({ default: m.AdminPage })))
const ProfilePage = lazy(() => import('./pages/ProfilePage').then((m) => ({ default: m.ProfilePage })))
const DlnaPage = lazy(() => import('./pages/DlnaPage').then((m) => ({ default: m.DlnaPage })))
const FileManagerPage = lazy(() =>
  import('./pages/FileManagerPage').then((m) => ({ default: m.FileManagerPage })),
)
const WatchHistoryPage = lazy(() =>
  import('./pages/WatchHistoryPage').then((m) => ({ default: m.WatchHistoryPage })),
)
const PosterWallPage = lazy(() => import('./pages/PosterWallPage').then((m) => ({ default: m.PosterWallPage })))
const ProfileManagementPage = lazy(() =>
  import('./pages/ProfileManagementPage').then((m) => ({ default: m.ProfileManagementPage })),
)
const SettingsPage = lazy(() => import('./pages/SettingsPage').then((m) => ({ default: m.SettingsPage })))
const StrmManagePage = lazy(() => import('./pages/StrmManagePage').then((m) => ({ default: m.StrmManagePage })))
const StrmDownloadQueuePage = lazy(() =>
  import('./pages/StrmQueuePage').then((m) => ({ default: m.StrmDownloadQueuePage })),
)
const StrmUploadQueuePage = lazy(() =>
  import('./pages/StrmQueuePage').then((m) => ({ default: m.StrmUploadQueuePage })),
)
const ScraperQueuePage = lazy(() =>
  import('./pages/ScraperQueuePage').then((m) => ({ default: m.ScraperQueuePage })),
)

export type AppRoute = {
  path?: string
  index?: boolean
  element: ReactElement
  adminOnly?: boolean
}

export const appRoutes: AppRoute[] = [
  { index: true, element: <HomePage /> },
  { path: 'libraries', element: <LibrariesPage /> },
  { path: 'library/:id', element: <LibraryPage /> },
  { path: 'favourites', element: <FavouritesPage /> },
  { path: 'playlists', element: <PlaylistsPage /> },
  { path: 'playlist/:id', element: <PlaylistDetailPage /> },
  { path: 'media/:id', element: <MediaDetailPage /> },
  { path: 'play/:id', element: <PlayerPage /> },
  { path: 'profile', element: <ProfilePage /> },
  { path: 'dlna', element: <DlnaPage /> },
  { path: 'history', element: <WatchHistoryPage /> },
  { path: 'poster-wall', element: <PosterWallPage /> },
  { path: 'play-profiles', element: <ProfileManagementPage /> },
  { path: 'api-configs', element: <Navigate to="/settings?group=api-configs" replace /> },
  { path: 'tools', element: <Navigate to="/files" replace /> },
  { path: 'files', element: <FileManagerPage />, adminOnly: true },
  { path: 'settings', element: <SettingsPage />, adminOnly: true },
  { path: 'strm', element: <StrmManagePage />, adminOnly: true },
  { path: 'strm/downloads', element: <StrmDownloadQueuePage />, adminOnly: true },
  { path: 'strm/uploads', element: <StrmUploadQueuePage />, adminOnly: true },
  { path: 'scraper/queue', element: <ScraperQueuePage />, adminOnly: true },
  { path: 'admin', element: <AdminPage />, adminOnly: true },
]
