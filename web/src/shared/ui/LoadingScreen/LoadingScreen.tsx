import { Spinner } from '@gfazioli/mantine-spinner'
import { Center, Stack } from '@mantine/core'

export function LoadingScreen({ height = '100%', size = 150 }: { height?: string; size?: number }) {
    return (
        <Center h={height}>
            <Stack align="center" gap="xs" w="100%">
                <Spinner inner={size / 3} segments={30} size={size} speed={1_900} strokeLinecap="butt" thickness={2} />
            </Stack>
        </Center>
    )
}
