import { ColorGradientStyle } from '@/shared/utils/config-parser'
import { ThemeIcon as MantineThemeIcon } from '@mantine/core'

interface IProps {
    getIconFromLibrary: (iconKey: string) => string
    gradientStyle: ColorGradientStyle
    isMobile: boolean
    svgIconColor: string
    svgIconKey: string
}
export const ThemeIcon = (props: IProps) => {
    const { isMobile, svgIconColor, gradientStyle, svgIconKey, getIconFromLibrary } = props

    return (
        <MantineThemeIcon
            color={svgIconColor}
            radius="xl"
            size={isMobile ? 36 : 44}
            style={{
                background: gradientStyle.background,
                border: gradientStyle.border,
                boxShadow: gradientStyle.boxShadow,
                flexShrink: 0,
            }}
            variant="light"
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
