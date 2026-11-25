import { renderSVG } from 'uqr'

import { useSubscription } from '@/entities/subscription-info-store'
import { useTranslation } from '@/shared/hooks'
import { constructSubscriptionUrl } from '@/shared/utils/construct-subscription-url'
import { vibrate } from '@/shared/utils/vibrate'
import { ActionIcon, Button, Group, Image, Stack, Text } from '@mantine/core'
import { useClipboard } from '@mantine/hooks'
import { modals } from '@mantine/modals'
import { notifications } from '@mantine/notifications'
import {
    IconBrandDiscord,
    IconBrandTelegram,
    IconBrandVk,
    IconCopy,
    IconLink,
    IconMessageChatbot,
} from '@tabler/icons-react'

interface IProps {
    hideGetLink: boolean
    supportUrl: string
}

export const SubscriptionLinkWidget = ({ supportUrl, hideGetLink }: IProps) => {
    const { t, baseTranslations } = useTranslation()
    const subscription = useSubscription()
    const clipboard = useClipboard({ timeout: 10000 })

    const subscriptionUrl = constructSubscriptionUrl(window.location.href, subscription.user.shortUuid)

    const handleCopy = () => {
        notifications.show({
            title: t(baseTranslations.linkCopied),
            message: t(baseTranslations.linkCopiedToClipboard),
            color: 'brand',
        })
        clipboard.copy(subscriptionUrl)
    }

    const renderSupportLink = (supportUrl: string) => {
        const iconConfig = {
            't.me': { icon: IconBrandTelegram, color: '#0088cc' },
            'discord.com': { icon: IconBrandDiscord, color: '#5865F2' },
            'vk.com': { icon: IconBrandVk, color: '#0077FF' },
        }

        const matchedPlatform = Object.entries(iconConfig).find(([domain]) => supportUrl.includes(domain))

        const { icon: Icon, color } = matchedPlatform
            ? matchedPlatform[1]
            : { icon: IconMessageChatbot, color: 'brand' }

        return (
            <ActionIcon
                c={color}
                component="a"
                href={supportUrl}
                radius="md"
                rel="noopener noreferrer"
                size="xl"
                target="_blank"
                variant="default"
            >
                <Icon />
            </ActionIcon>
        )
    }

    const handleGetLink = () => {
        vibrate('tap')

        const subscriptionQrCode = renderSVG(subscriptionUrl, {
            whiteColor: '#ffffff',
            blackColor: '#09090b',
        })

        modals.open({
            centered: true,
            title: t(baseTranslations.getLink),
            children: (
                <Stack align="center">
                    <Image
                        src={`data:image/svg+xml;utf8,${encodeURIComponent(subscriptionQrCode)}`}
                        style={{ borderRadius: 'var(--mantine-radius-md)' }}
                    />
                    <Text c="var(--app-text-strong)" fw={600} size="lg" ta="center">
                        {t(baseTranslations.scanQrCode)}
                    </Text>
                    <Text c="dimmed" size="sm" ta="center">
                        {t(baseTranslations.scanQrCodeDescription)}
                    </Text>

                    <Button fullWidth leftSection={<IconCopy />} onClick={handleCopy} radius="md">
                        {t(baseTranslations.copyLink)}
                    </Button>
                </Stack>
            ),
        })
    }

    return (
        <Group gap="xs" ml="auto" wrap="nowrap">
            {!hideGetLink && (
                <ActionIcon onClick={handleGetLink} radius="md" size="xl" variant="default">
                    <IconLink />
                </ActionIcon>
            )}

            {supportUrl !== '' && renderSupportLink(supportUrl)}
        </Group>
    )
}
