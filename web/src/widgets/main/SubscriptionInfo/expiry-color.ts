export type ExpiryColor = 'gray' | 'orange' | 'red'

// Only a subscription that is about to end (or has ended) gets a colour.
export const getExpiryColor = (daysLeft: number): ExpiryColor => {
    if (daysLeft <= 0) return 'red'
    if (daysLeft <= 3) return 'orange'
    return 'gray'
}
