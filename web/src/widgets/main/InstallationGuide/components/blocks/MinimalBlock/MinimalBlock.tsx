import { ThemeIcon } from '@/shared/ui/ThemeIcon'
import { getColorGradient, getLocalizedText } from '@/shared/utils/config-parser'
import { Box, Group, Stack, Text } from '@mantine/core'

import { IBlockRendererProps } from '../rendererBlock.types'

import classes from './MinimalBlock.module.css'

export const MinimalBlockRenderer = ({
    blocks,
    isMobile,
    currentLang,
    renderBlockButtons,
    getIconFromLibrary,
}: IBlockRendererProps) => {
    return (
        <Stack gap="md">
            {blocks.map((block, index) => {
                const gradientStyle = getColorGradient(block.svgIconColor)

                return (
                    <Box className={classes.stepBlock} key={index}>
                        <Group gap="sm" mb="xs" wrap="nowrap">
                            <ThemeIcon
                                getIconFromLibrary={getIconFromLibrary}
                                gradientStyle={gradientStyle}
                                isMobile={isMobile}
                                svgIconColor={block.svgIconColor}
                                svgIconKey={block.svgIconKey}
                            />
                            <Text
                                c="white"
                                dangerouslySetInnerHTML={{
                                    __html: getLocalizedText(block.title, currentLang),
                                }}
                                fw={500}
                                size={isMobile ? 'sm' : 'md'}
                            />
                        </Group>
                        <Text
                            c="dimmed"
                            dangerouslySetInnerHTML={{
                                __html: getLocalizedText(block.description, currentLang),
                            }}
                            size={isMobile ? 'xs' : 'sm'}
                            style={{ lineHeight: 1.6 }}
                        />
                        {block.buttons.length > 0 && (
                            <Box style={{ marginTop: 8 }}>{renderBlockButtons(block.buttons, 'subtle')}</Box>
                        )}
                    </Box>
                )
            })}
        </Stack>
    )
}
