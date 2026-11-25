// Share of the traffic limit already used, 0 to 100. Null when there is no
// limit to measure against (unlimited plans report a zero limit).
export const getTrafficUsagePercent = (usedBytes: string, limitBytes: string): null | number => {
    const used = Number(usedBytes)
    const limit = Number(limitBytes)

    if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return null

    return Math.min(100, Math.max(0, (used / limit) * 100))
}

// The bar only changes colour when the limit is close, like the expiry date.
export const getTrafficUsageColor = (percent: number): 'brand' | 'orange' | 'red' => {
    if (percent >= 100) return 'red'
    if (percent >= 80) return 'orange'
    return 'brand'
}
