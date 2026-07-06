import { TSubscriptionPageLanguageCode, TSubscriptionPageRawConfig } from '@remnawave/subscription-page-types'

export interface IState {
    config: null | TSubscriptionPageRawConfig
    currentLang: TSubscriptionPageLanguageCode
    isConfigLoaded: boolean
}

export interface IActions {
    actions: {
        getInitialState: () => IState
        resetState: () => Promise<void>
        setConfig: (config: TSubscriptionPageRawConfig) => void
        setLanguage: (lang: TSubscriptionPageLanguageCode) => void
    }
}
