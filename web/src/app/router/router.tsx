import { createBrowserRouter, createRoutesFromElements, Route, RouterProvider } from 'react-router'

import { RootLayout } from '@/app/layouts/RootLayout'
import { ServerError } from '@/pages/errors/ServerError'
import { MainPageConnector } from '@/pages/main/ui/connectors/MainPageConnector'
import { ErrorBoundary } from '@/shared/hocs/ErrorBoundary'

const router = createBrowserRouter(
    createRoutesFromElements(
        <Route element={<ErrorBoundary fallback={<ServerError />} />}>
            <Route element={<RootLayout />} path="*">
                <Route element={<MainPageConnector />} path="*" />
            </Route>
        </Route>
    )
)

export function Router() {
    return <RouterProvider router={router} />
}
