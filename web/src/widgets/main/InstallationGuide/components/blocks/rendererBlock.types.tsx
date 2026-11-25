import {
    TSubscriptionPageBlockConfig,
    TSubscriptionPageButtonConfig,
    TSubscriptionPageLanguageCode,
} from '@remnawave/subscription-page-types'

export interface IBlockRendererProps {
    blocks: TSubscriptionPageBlockConfig[]
    currentLang: TSubscriptionPageLanguageCode
    getIconFromLibrary: (iconKey: string) => string
    isMobile: boolean
    renderBlockButtons: (buttons: TSubscriptionPageButtonConfig[]) => React.ReactNode
    svgLibrary: Record<string, string>
}
