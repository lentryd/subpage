import { FC } from 'react'
import { ErrorBoundary as ReactErrorBoundary, ErrorBoundaryProps } from 'react-error-boundary'
import { Outlet } from 'react-router'

export const ErrorBoundary: FC<ErrorBoundaryProps> = (props) => {
    return (
        <ReactErrorBoundary {...props}>
            <Outlet />
        </ReactErrorBoundary>
    )
}
