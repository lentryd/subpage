import { useSubscription } from '@/entities/subscription-info-store'
import { useTranslation } from '@/shared/hooks'
import { formatDate } from '@/shared/utils/config-parser'
import { getTrafficUsageColor, getTrafficUsagePercent } from '@/shared/utils/traffic-usage'
import { Box, Group, Progress, SimpleGrid, Stack, Text, ThemeIcon } from '@mantine/core'
import { IconArrowsUpDown, IconCalendar, IconCheck, IconUserScan, IconX } from '@tabler/icons-react'

import { getExpiryColor } from './expiry-color'

import classes from './SubscriptionInfoCards.module.css'

type ColorVariant = 'gray' | 'green' | 'orange' | 'red'

interface CardItemProps {
    color: ColorVariant
    icon: React.ReactNode
    label: string
    progress?: null | number
    value: string
}

const CardItem = ({ icon, label, progress, value, color }: CardItemProps) => {
    return (
        <Box className={classes.cardItem}>
            <Group gap="xs" wrap="nowrap">
                <ThemeIcon color={color} radius="md" size={36} style={{ flexShrink: 0 }} variant="soft">
                    {icon}
                </ThemeIcon>
                <Stack gap={2} style={{ minWidth: 0, flex: 1 }}>
                    <Text c="dimmed" className={classes.label} fw={500} lh={1} size="xs">
                        {label}
                    </Text>
                    <Text c="var(--app-text-strong)" className={`${classes.value} num`} fw={600} size="sm">
                        {value}
                    </Text>
                    {typeof progress === 'number' && (
                        <Progress
                            aria-label={label}
                            color={getTrafficUsageColor(progress)}
                            mt={4}
                            size={6}
                            value={progress}
                        />
                    )}
                </Stack>
            </Group>
        </Box>
    )
}

interface IProps {
    isMobile: boolean
}

export const SubscriptionInfoCardsWidget = ({ isMobile: _ }: IProps) => {
    const { t, currentLang, baseTranslations } = useTranslation()
    const subscription = useSubscription()

    const { user } = subscription

    const isActive = user.userStatus === 'ACTIVE'
    const statusText = isActive ? t(baseTranslations.active) : t(baseTranslations.inactive)

    const bandwidthValue =
        user.trafficLimit === '0' ? `${user.trafficUsed} / ∞` : `${user.trafficUsed} / ${user.trafficLimit}`

    return (
        <SimpleGrid cols={{ base: 1, xs: 1, sm: 2 }} spacing="xs" verticalSpacing="xs">
            <CardItem
                color="gray"
                icon={<IconUserScan size={18} />}
                label={t(baseTranslations.name)}
                value={user.username}
            />

            <CardItem
                color={isActive ? 'green' : 'red'}
                icon={isActive ? <IconCheck size={18} /> : <IconX size={18} />}
                label={t(baseTranslations.status)}
                value={statusText}
            />

            <CardItem
                color={getExpiryColor(user.daysLeft)}
                icon={<IconCalendar size={18} />}
                label={t(baseTranslations.expires)}
                value={formatDate(user.expiresAt, currentLang, baseTranslations)}
            />

            <CardItem
                color="gray"
                icon={<IconArrowsUpDown size={18} />}
                label={t(baseTranslations.bandwidth)}
                progress={getTrafficUsagePercent(user.trafficUsedBytes, user.trafficLimitBytes)}
                value={bandwidthValue}
            />
        </SimpleGrid>
    )
}
