import { getTrafficUsageColor } from '@/shared/utils/traffic-usage'
import { Box, Group, Progress, Stack, Text } from '@mantine/core'

import { IInfoBlockProps } from './InfoBlock.types'

import classes from './InfoBlock.module.css'

export const InfoBlock = ({ color, icon, progress, title, value }: IInfoBlockProps) => {
    return (
        <Box className={classes.infoBlock}>
            <Stack gap={6}>
                <Group gap={6} wrap="nowrap">
                    <Box
                        c={color === 'gray' ? 'dimmed' : `var(--mantine-color-${color}-light-color)`}
                        style={{ display: 'flex', flexShrink: 0 }}
                    >
                        {icon}
                    </Box>
                    <Text c="dimmed" fw={500} size="xs" truncate>
                        {title}
                    </Text>
                </Group>
                <Text c="var(--app-text-strong)" className="num" fw={600} size="md" truncate>
                    {value}
                </Text>
                {typeof progress === 'number' && (
                    <Progress
                        aria-label={title}
                        color={getTrafficUsageColor(progress)}
                        mt={4}
                        size={6}
                        value={progress}
                    />
                )}
            </Stack>
        </Box>
    )
}
