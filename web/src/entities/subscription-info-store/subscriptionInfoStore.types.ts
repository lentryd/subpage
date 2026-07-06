import { GetSubscriptionInfoByShortUuidCommand } from '@remnawave/backend-contract'

export interface IState {
    subscription: GetSubscriptionInfoByShortUuidCommand.Response['response'] | null
}

export interface IActions {
    actions: {
        getInitialState: () => IState
        resetState: () => Promise<void>
        setSubscriptionInfo: (info: IState) => void
    }
}
