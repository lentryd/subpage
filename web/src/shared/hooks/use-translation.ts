import { useCallback } from 'react'

import { useAppConfig, useCurrentLang } from '@/entities/app-config-store'
import { getLocalizedText } from '@/shared/utils/config-parser'
import { TSubscriptionPageLocalizedText } from '@remnawave/subscription-page-types'

export const useTranslation = () => {
    const config = useAppConfig()
    const currentLang = useCurrentLang()

    const t = useCallback(
        (textObj: TSubscriptionPageLocalizedText) => getLocalizedText(textObj, currentLang),
        [currentLang]
    )

    return {
        t,
        currentLang,
        baseTranslations: config.baseTranslations,
    }
}
