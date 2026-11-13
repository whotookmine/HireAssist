import { Alert, Badge, Button, Group, Modal, NumberInput, Paper, SegmentedControl, Select, Stack, Table, Tabs, Text, TextInput, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconInfoCircle, IconRefresh, IconTrash } from '@tabler/icons-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  eraseCandidate,
  getRetentionPolicy,
  listAccessRejections,
  listErasures,
  listHeldExpiries,
  resolveHeldExpiry,
  runRetentionSweep,
  setRetentionPolicy,
} from '../api/endpoints'
import type { Erasure, ExpiryAction, HoldDecision, RetentionAnchor, RetentionPolicy } from '../api/types'
import { useAuth } from '../auth/AuthProvider'
import { PageLoader, QueryError } from '../components/common'
import { useI18n } from '../i18n/I18nProvider'

/** UC-5: held expiries for every member; the policy, erasures and the access audit for Admins. */
export function RetentionPage() {
  const { t } = useI18n()
  const { member } = useAuth()
  const isAdmin = member?.role === 'admin'
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = searchParams.get('tab') ?? 'held'

  return (
    <>
      <Title order={2} mb="xs">
        {t('retention.title')}
      </Title>
      <Text c="dimmed" size="sm" mb="md">
        {t('retention.subtitle')}
      </Text>
      <Tabs value={tab} onChange={(value) => setSearchParams(value === 'held' ? {} : { tab: value ?? '' })}>
        <Tabs.List>
          <Tabs.Tab value="held">{t('retention.tab.held')}</Tabs.Tab>
          {isAdmin && <Tabs.Tab value="policy">{t('retention.tab.policy')}</Tabs.Tab>}
          {isAdmin && <Tabs.Tab value="erasures">{t('retention.tab.erasures')}</Tabs.Tab>}
          {isAdmin && <Tabs.Tab value="audit">{t('retention.tab.audit')}</Tabs.Tab>}
        </Tabs.List>
        <Tabs.Panel value="held" pt="md">
          <HeldExpiries />
        </Tabs.Panel>
        {isAdmin && (
          <>
            <Tabs.Panel value="policy" pt="md">
              <PolicyForm />
            </Tabs.Panel>
            <Tabs.Panel value="erasures" pt="md">
              <Erasures />
            </Tabs.Panel>
            <Tabs.Panel value="audit" pt="md">
              <AccessAudit />
            </Tabs.Panel>
          </>
        )}
      </Tabs>
    </>
  )
}

function Empty({ text }: { text: string }) {
  return (
    <Paper withBorder p="xl">
      <Text c="dimmed" ta="center">
        {text}
      </Text>
    </Paper>
  )
}

/** Tells the member how an erasure ended: erased, or still pending at a named holder (FR-5.8). */
function useErasureNotice() {
  const { t } = useI18n()
  return (e: Erasure) =>
    e.status === 'erased'
      ? notifications.show({ color: 'green', message: t('retention.erased', { ref: e.ref }) })
      : notifications.show({ color: 'yellow', message: t('retention.pending', { ref: e.ref }) })
}

/** FR-5.6: an expiry held because the candidate is in an active hiring process, for a decision. */
function HeldExpiries() {
  const { t, formatDate } = useI18n()
  const queryClient = useQueryClient()
  const notify = useErasureNotice()
  const [confirming, setConfirming] = useState<string | null>(null)
  const held = useQuery({ queryKey: ['held-expiries'], queryFn: listHeldExpiries })
  const resolve = useMutation({
    mutationFn: ({ candidateId, decision }: { candidateId: string; decision: HoldDecision }) => resolveHeldExpiry(candidateId, decision),
    onSuccess: (answer) => {
      setConfirming(null)
      if ('ref' in answer) notify(answer)
      else notifications.show({ message: t('retention.kept') })
      queryClient.invalidateQueries({ queryKey: ['held-expiries'] })
      queryClient.invalidateQueries({ queryKey: ['erasures'] })
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })

  if (held.isPending) return <PageLoader />
  if (held.isError) return <QueryError error={held.error} onRetry={() => held.refetch()} />
  if (held.data.length === 0) return <Empty text={t('retention.heldEmpty')} />

  return (
    <>
      <Paper withBorder>
        <Table verticalSpacing="sm">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('retention.col.candidate')}</Table.Th>
              <Table.Th>{t('pipeline.col.opening')}</Table.Th>
              <Table.Th>{t('retention.col.expiredOn')}</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {held.data.map((h) => (
              <Table.Tr key={h.candidate_id}>
                <Table.Td ff="monospace">{h.candidate_id}</Table.Td>
                <Table.Td>
                  <Link to={`/job-openings/${h.job_opening_id}`}>{h.job_opening_id}</Link>
                </Table.Td>
                <Table.Td>{formatDate(h.expires_at)}</Table.Td>
                <Table.Td>
                  <Group gap="xs" justify="flex-end" wrap="nowrap">
                    <Button
                      size="xs"
                      variant="default"
                      onClick={() => resolve.mutate({ candidateId: h.candidate_id, decision: 'keep' })}
                      aria-label={t('retention.keepFor', { id: h.candidate_id })}
                    >
                      {t('retention.keep')}
                    </Button>
                    <Button size="xs" color="red" variant="light" onClick={() => setConfirming(h.candidate_id)}>
                      {t('retention.erase')}
                    </Button>
                  </Group>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>
      <ConfirmErase
        candidateId={confirming}
        loading={resolve.isPending}
        onCancel={() => setConfirming(null)}
        onConfirm={(candidateId) => resolve.mutate({ candidateId, decision: 'erase' })}
      />
    </>
  )
}

function ConfirmErase(props: { candidateId: string | null; loading: boolean; onCancel: () => void; onConfirm: (id: string) => void }) {
  const { t } = useI18n()
  return (
    <Modal opened={props.candidateId !== null} onClose={props.onCancel} title={t('retention.confirmTitle')} centered>
      <Text size="sm" mb="md">
        {t('retention.confirmBody', { id: props.candidateId ?? '' })}
      </Text>
      <Group justify="flex-end">
        <Button variant="default" onClick={props.onCancel}>
          {t('common.cancel')}
        </Button>
        <Button color="red" leftSection={<IconTrash size={16} />} loading={props.loading} onClick={() => props.onConfirm(props.candidateId!)}>
          {t('retention.confirmErase')}
        </Button>
      </Group>
    </Modal>
  )
}

const DEFAULT_POLICY: RetentionPolicy = { period_days: 365, anchor: 'last_activity', expiry_action: 'delete', warning_window_days: 30 }

/** FR-5.1. There is no default policy: until one is saved, nothing is ever erased on expiry. */
function PolicyForm() {
  const { t, formatDate } = useI18n()
  const queryClient = useQueryClient()
  const policy = useQuery({ queryKey: ['retention-policy'], queryFn: getRetentionPolicy })
  const [draft, setDraft] = useState<RetentionPolicy | null>(null)
  const save = useMutation({
    mutationFn: setRetentionPolicy,
    onSuccess: (saved) => {
      queryClient.setQueryData(['retention-policy'], saved)
      setDraft(null)
      notifications.show({ color: 'green', message: t('retention.policySaved') })
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })
  const sweep = useMutation({
    mutationFn: runRetentionSweep,
    onSuccess: (r) => {
      notifications.show({
        message: r.policy_set
          ? t('retention.swept', { evaluated: r.evaluated, erased: r.erasures.length, held: r.held.length, warned: r.warned.length })
          : t('retention.sweptNoPolicy'),
      })
      queryClient.invalidateQueries({ queryKey: ['held-expiries'] })
      queryClient.invalidateQueries({ queryKey: ['erasures'] })
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })

  if (policy.isPending) return <PageLoader />
  if (policy.isError) return <QueryError error={policy.error} onRetry={() => policy.refetch()} />
  const value = draft ?? policy.data ?? DEFAULT_POLICY
  const edit = (change: Partial<RetentionPolicy>) => setDraft({ ...value, ...change })
  const submit = (e: FormEvent) => {
    e.preventDefault()
    save.mutate({ period_days: value.period_days, anchor: value.anchor, expiry_action: value.expiry_action, warning_window_days: value.warning_window_days })
  }

  return (
    <Paper withBorder p="md" maw={560}>
      <form onSubmit={submit}>
        <Stack>
          {policy.data ? (
            <Text size="sm" c="dimmed">
              {t('retention.policyUpdated', { date: formatDate(policy.data.updated_at ?? '') })}
            </Text>
          ) : (
            <Alert color="yellow" icon={<IconInfoCircle size={16} />}>
              {t('retention.noPolicy')}
            </Alert>
          )}
          <NumberInput
            label={t('retention.period')}
            min={1}
            value={value.period_days}
            onChange={(v) => edit({ period_days: Number(v) || 0 })}
            suffix={` ${t('retention.daysUnit')}`}
          />
          <Select
            label={t('retention.anchor')}
            value={value.anchor}
            allowDeselect={false}
            onChange={(v) => edit({ anchor: v as RetentionAnchor })}
            data={[
              { value: 'collected', label: t('retention.anchor.collected') },
              { value: 'last_activity', label: t('retention.anchor.last_activity') },
            ]}
          />
          <div>
            <Text size="sm" fw={500} mb={4}>
              {t('retention.action')}
            </Text>
            <SegmentedControl
              value={value.expiry_action}
              onChange={(v) => edit({ expiry_action: v as ExpiryAction })}
              data={[
                { value: 'delete', label: t('retention.action.delete') },
                { value: 'anonymise', label: t('retention.action.anonymise') },
              ]}
            />
          </div>
          <NumberInput
            label={t('retention.warning')}
            description={t('retention.warningHint')}
            min={0}
            value={value.warning_window_days}
            onChange={(v) => edit({ warning_window_days: Number(v) || 0 })}
            suffix={` ${t('retention.daysUnit')}`}
          />
          <Group justify="space-between">
            <Button variant="default" leftSection={<IconRefresh size={16} />} loading={sweep.isPending} onClick={() => sweep.mutate()}>
              {t('retention.sweep')}
            </Button>
            <Button type="submit" loading={save.isPending} disabled={policy.data !== null && draft === null}>
              {t('retention.savePolicy')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Paper>
  )
}

const ERASURE_COLOR = { erased: 'green', pending: 'yellow' } as const

/** FR-5.7 erasure on request, and the FR-5.5 audit entries: by reference, never by person. */
function Erasures() {
  const { t, formatDate } = useI18n()
  const queryClient = useQueryClient()
  const notify = useErasureNotice()
  const [candidateId, setCandidateId] = useState('')
  const [confirming, setConfirming] = useState<string | null>(null)
  const erasures = useQuery({ queryKey: ['erasures'], queryFn: () => listErasures() })
  const erase = useMutation({
    mutationFn: eraseCandidate,
    onSuccess: (e) => {
      setConfirming(null)
      setCandidateId('')
      notify(e)
      queryClient.invalidateQueries({ queryKey: ['erasures'] })
      queryClient.invalidateQueries({ queryKey: ['held-expiries'] })
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })

  return (
    <Stack>
      <Paper withBorder p="md">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (candidateId.trim()) setConfirming(candidateId.trim())
          }}
        >
          <Group align="flex-end">
            <TextInput
              label={t('retention.eraseNow')}
              description={t('retention.eraseNowHint')}
              placeholder="cand-001"
              value={candidateId}
              onChange={(e) => setCandidateId(e.currentTarget.value)}
              style={{ flex: 1 }}
            />
            <Button type="submit" color="red" variant="light" disabled={!candidateId.trim()}>
              {t('retention.erase')}
            </Button>
          </Group>
        </form>
      </Paper>

      {erasures.isPending ? (
        <PageLoader />
      ) : erasures.isError ? (
        <QueryError error={erasures.error} onRetry={() => erasures.refetch()} />
      ) : erasures.data.length === 0 ? (
        <Empty text={t('retention.erasuresEmpty')} />
      ) : (
        <Paper withBorder>
          <Table verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('retention.col.ref')}</Table.Th>
                <Table.Th>{t('openings.col.status')}</Table.Th>
                <Table.Th>{t('retention.col.reason')}</Table.Th>
                <Table.Th>{t('retention.action')}</Table.Th>
                <Table.Th>{t('retention.col.holders')}</Table.Th>
                <Table.Th>{t('retention.col.requested')}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {erasures.data.map((e) => (
                <Table.Tr key={e.ref}>
                  <Table.Td ff="monospace">{e.ref}</Table.Td>
                  <Table.Td>
                    <Badge color={ERASURE_COLOR[e.status]} variant="light">
                      {t(`retention.status.${e.status}`)}
                    </Badge>
                  </Table.Td>
                  <Table.Td>{t(`retention.reason.${e.reason}`)}</Table.Td>
                  <Table.Td>{t(`retention.action.${e.expiry_action}`)}</Table.Td>
                  <Table.Td>
                    <Group gap={4}>
                      {Object.entries(e.holders).map(([holder, status]) => (
                        <Badge key={holder} size="sm" variant="outline" color={ERASURE_COLOR[status]}>
                          {t(`retention.holder.${holder as keyof Erasure['holders']}`)}
                        </Badge>
                      ))}
                    </Group>
                    {e.last_error && (
                      <Text size="xs" c="red">
                        {e.last_error}
                      </Text>
                    )}
                  </Table.Td>
                  <Table.Td>{formatDate(e.requested_at)}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Paper>
      )}
      <ConfirmErase candidateId={confirming} loading={erase.isPending} onCancel={() => setConfirming(null)} onConfirm={(id) => erase.mutate(id)} />
    </Stack>
  )
}

/** FR-0.4: every request the gateway refused for want of a role. */
function AccessAudit() {
  const { t, formatDate } = useI18n()
  const audit = useQuery({ queryKey: ['access-audit'], queryFn: listAccessRejections })
  if (audit.isPending) return <PageLoader />
  if (audit.isError) return <QueryError error={audit.error} onRetry={() => audit.refetch()} />
  if (audit.data.length === 0) return <Empty text={t('retention.auditEmpty')} />
  return (
    <Paper withBorder>
      <Table verticalSpacing="sm">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('retention.col.when')}</Table.Th>
            <Table.Th>{t('retention.col.member')}</Table.Th>
            <Table.Th>{t('retention.col.role')}</Table.Th>
            <Table.Th>{t('retention.col.request')}</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {audit.data.map((r) => (
            <Table.Tr key={r.id}>
              <Table.Td>{formatDate(r.occurred_at)}</Table.Td>
              <Table.Td ff="monospace">{r.member_id}</Table.Td>
              <Table.Td>{t('retention.roleNeeded', { role: r.role, required: r.required_role })}</Table.Td>
              <Table.Td ff="monospace">
                {r.method} {r.path}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Paper>
  )
}
