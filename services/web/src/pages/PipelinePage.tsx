import { Badge, Button, Group, Paper, Table, Text, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconRefresh } from '@tabler/icons-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getPipelineDashboard, runStalenessSweep } from '../api/endpoints'
import { useAuth } from '../auth/AuthProvider'
import { PageLoader, QueryError, StatusBadge } from '../components/common'
import { useI18n } from '../i18n/I18nProvider'

/** UC-4: counts per stage, days open against the expected time-to-fill, and the stale flag (FR-4.1). */
export function PipelinePage() {
  const { t } = useI18n()
  const { member } = useAuth()
  const queryClient = useQueryClient()
  const dashboard = useQuery({ queryKey: ['pipeline'], queryFn: getPipelineDashboard })
  const sweep = useMutation({
    mutationFn: runStalenessSweep,
    onSuccess: (report) => {
      notifications.show({ message: t('pipeline.swept', { flagged: report.flagged.length, cleared: report.cleared.length }) })
      queryClient.invalidateQueries({ queryKey: ['pipeline'] })
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })

  return (
    <>
      <Group justify="space-between" mb="xs">
        <Title order={2}>{t('pipeline.title')}</Title>
        {member?.role === 'admin' && (
          <Button variant="default" leftSection={<IconRefresh size={16} />} loading={sweep.isPending} onClick={() => sweep.mutate()}>
            {t('pipeline.sweep')}
          </Button>
        )}
      </Group>
      <Text c="dimmed" size="sm" mb="md">
        {t('pipeline.subtitle')}
      </Text>

      {dashboard.isPending ? (
        <PageLoader />
      ) : dashboard.isError ? (
        <QueryError error={dashboard.error} onRetry={() => dashboard.refetch()} />
      ) : dashboard.data.length === 0 ? (
        <Paper withBorder p="xl">
          <Text c="dimmed" ta="center">
            {t('pipeline.empty')}
          </Text>
        </Paper>
      ) : (
        <Paper withBorder>
          <Table verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('pipeline.col.opening')}</Table.Th>
                <Table.Th>{t('openings.col.status')}</Table.Th>
                <Table.Th ta="right">{t('pipeline.col.applied')}</Table.Th>
                <Table.Th ta="right">{t('pipeline.col.screened')}</Table.Th>
                <Table.Th ta="right">{t('pipeline.col.shortlisted')}</Table.Th>
                <Table.Th>{t('pipeline.col.daysOpen')}</Table.Th>
                <Table.Th>{t('pipeline.col.flag')}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {dashboard.data.map((o) => (
                <Table.Tr key={o.job_opening_id}>
                  <Table.Td>
                    <Text fw={600} component={Link} to={`/job-openings/${o.job_opening_id}`} c="inherit" td="none">
                      {o.title || o.job_opening_id}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <StatusBadge opening={o} />
                  </Table.Td>
                  <Table.Td ta="right">{o.applied}</Table.Td>
                  <Table.Td ta="right">{o.screened}</Table.Td>
                  <Table.Td ta="right">{o.shortlisted}</Table.Td>
                  <Table.Td>{t('pipeline.daysOf', { days: o.days_open, expected: o.expected_time_to_fill_days })}</Table.Td>
                  <Table.Td>
                    {o.flagged ? (
                      <Badge color="red" variant="light">
                        {t('pipeline.stale')}
                      </Badge>
                    ) : o.status === 'paused' ? (
                      <Text size="sm" c="dimmed">
                        {t('pipeline.pausedExcluded')}
                      </Text>
                    ) : (
                      <Badge color="green" variant="outline">
                        {t('pipeline.onTrack')}
                      </Badge>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Paper>
      )}
    </>
  )
}
