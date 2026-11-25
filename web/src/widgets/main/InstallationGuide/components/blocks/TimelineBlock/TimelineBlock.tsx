import { ThemeIcon } from '@/shared/ui/ThemeIcon'
import { getLocalizedText } from '@/shared/utils/config-parser'
import { Stack, Text, Timeline } from '@mantine/core'

import { IBlockRendererProps } from '../rendererBlock.types'

import classes from './TimelineBlock.module.css'

export const TimelineBlockRenderer = ({
    blocks,
    isMobile,
    currentLang,
    renderBlockButtons,
    getIconFromLibrary,
}: IBlockRendererProps) => {
    return (
        <Timeline
            active={blocks.length}
            bulletSize={isMobile ? 36 : 44}
            classNames={{
                root: classes.timelineRoot,
                item: classes.timelineItem,
                itemBullet: classes.timelineItemBullet,
            }}
            color="gray"
            lineWidth={2}
        >
            {blocks.map((block, index) => {
                return (
                    <Timeline.Item
                        bullet={
                            <ThemeIcon
                                getIconFromLibrary={getIconFromLibrary}
                                isMobile={isMobile}
                                svgIconKey={block.svgIconKey}
                            />
                        }
                        key={index}
                        title={
                            <Text
                                c="var(--app-text-strong)"
                                dangerouslySetInnerHTML={{
                                    __html: getLocalizedText(block.title, currentLang),
                                }}
                                fw={600}
                                size={isMobile ? 'sm' : 'md'}
                            />
                        }
                    >
                        <Stack gap="xs">
                            <Text
                                c="dimmed"
                                dangerouslySetInnerHTML={{
                                    __html: getLocalizedText(block.description, currentLang),
                                }}
                                size={isMobile ? 'xs' : 'sm'}
                                style={{ lineHeight: 1.6 }}
                            />
                            {renderBlockButtons(block.buttons)}
                        </Stack>
                    </Timeline.Item>
                )
            })}
        </Timeline>
    )
}
