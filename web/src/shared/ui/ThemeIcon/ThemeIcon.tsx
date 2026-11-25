import { ThemeIcon as MantineThemeIcon } from '@mantine/core'

interface IProps {
    getIconFromLibrary: (iconKey: string) => string
    isMobile: boolean
    svgIconKey: string
}
export const ThemeIcon = (props: IProps) => {
    const { isMobile, svgIconKey, getIconFromLibrary } = props

    return (
        <MantineThemeIcon
            color="gray"
            radius="xl"
            size={isMobile ? 36 : 44}
            // opaque, so the timeline line never shows through the bullet
            style={{
                background: 'linear-gradient(var(--app-fill), var(--app-fill)), var(--app-surface)',
                flexShrink: 0,
            }}
            variant="soft"
        >
            <span
                dangerouslySetInnerHTML={{
                    __html: getIconFromLibrary(svgIconKey),
                }}
                style={{ display: 'flex', alignItems: 'center' }}
            />
        </MantineThemeIcon>
    )
}
