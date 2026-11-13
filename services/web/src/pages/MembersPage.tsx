import { Badge, Button, Group, Modal, Paper, PasswordInput, SegmentedControl, Select, Stack, Table, Text, TextInput, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconKey, IconPlus, IconUserMinus } from '@tabler/icons-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { changeMemberRole, createMember, listMembers, removeMember, setMemberPassword } from '../api/endpoints'
import type { Member, Role } from '../api/types'
import { useAuth } from '../auth/AuthProvider'
import { PageLoader, QueryError } from '../components/common'
import { useI18n } from '../i18n/I18nProvider'

const MIN_PASSWORD = 15

/** UC-6: an Admin adds, removes and re-roles members, and resets passwords (FR-6.1–FR-6.5). */
export function MembersPage() {
  const { t, formatDate } = useI18n()
  const { member: me } = useAuth()
  const queryClient = useQueryClient()
  const members = useQuery({ queryKey: ['members'], queryFn: listMembers })
  const [adding, setAdding] = useState(false)
  const [resetting, setResetting] = useState<Member | null>(null)
  const [removing, setRemoving] = useState<Member | null>(null)

  const done = (message: string) => {
    notifications.show({ color: 'green', message })
    queryClient.invalidateQueries({ queryKey: ['members'] })
  }
  const failed = (err: Error) => notifications.show({ color: 'red', message: err.message })

  const changeRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: Role }) => changeMemberRole(id, role),
    onSuccess: (m) => done(t('members.roleChanged', { email: m.email, role: t(`role.${m.role}`) })),
    onError: failed,
  })
  const remove = useMutation({
    mutationFn: (m: Member) => removeMember(m.id),
    onSuccess: (_, m) => {
      setRemoving(null)
      done(t('members.removed', { email: m.email }))
    },
    onError: failed,
  })

  return (
    <>
      <Group justify="space-between" mb="xs">
        <Title order={2}>{t('members.title')}</Title>
        <Button leftSection={<IconPlus size={16} />} onClick={() => setAdding(true)}>
          {t('members.add')}
        </Button>
      </Group>
      <Text c="dimmed" size="sm" mb="md">
        {t('members.subtitle')}
      </Text>

      {members.isPending ? (
        <PageLoader />
      ) : members.isError ? (
        <QueryError error={members.error} onRetry={() => members.refetch()} />
      ) : (
        <Paper withBorder>
          <Table verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('signIn.email')}</Table.Th>
                <Table.Th>{t('members.col.role')}</Table.Th>
                <Table.Th visibleFrom="sm">{t('members.col.added')}</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {members.data.map((m) => (
                <Table.Tr key={m.id}>
                  <Table.Td>
                    <Group gap="xs">
                      <Text size="sm">{m.email}</Text>
                      {m.id === me?.id && (
                        <Badge size="xs" variant="light">
                          {t('members.you')}
                        </Badge>
                      )}
                      {m.temporary_password && (
                        <Badge size="xs" color="yellow" variant="light">
                          {t('members.temporary')}
                        </Badge>
                      )}
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    <Select
                      size="xs"
                      w={140}
                      aria-label={t('members.roleOf', { email: m.email })}
                      value={m.role}
                      allowDeselect={false}
                      disabled={changeRole.isPending}
                      onChange={(role) => role && role !== m.role && changeRole.mutate({ id: m.id, role: role as Role })}
                      data={[
                        { value: 'recruiter', label: t('role.recruiter') },
                        { value: 'admin', label: t('role.admin') },
                      ]}
                    />
                  </Table.Td>
                  <Table.Td visibleFrom="sm">{formatDate(m.created_at)}</Table.Td>
                  <Table.Td>
                    <Group gap="xs" justify="flex-end" wrap="nowrap">
                      <Button size="xs" variant="default" leftSection={<IconKey size={14} />} onClick={() => setResetting(m)}>
                        {t('members.resetPassword')}
                      </Button>
                      <Button
                        size="xs"
                        color="red"
                        variant="light"
                        leftSection={<IconUserMinus size={14} />}
                        onClick={() => setRemoving(m)}
                        aria-label={t('members.removeOf', { email: m.email })}
                      >
                        {t('members.remove')}
                      </Button>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Paper>
      )}

      <AddMember opened={adding} onClose={() => setAdding(false)} onAdded={(m) => done(t('members.added', { email: m.email }))} />
      <ResetPassword member={resetting} onClose={() => setResetting(null)} onReset={(m) => done(t('members.passwordReset', { email: m.email }))} />
      <Modal opened={removing !== null} onClose={() => setRemoving(null)} title={t('members.removeTitle')} centered>
        <Text size="sm" mb="md">
          {t('members.removeBody', { email: removing?.email ?? '' })}
        </Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={() => setRemoving(null)}>
            {t('common.cancel')}
          </Button>
          <Button color="red" loading={remove.isPending} onClick={() => removing && remove.mutate(removing)}>
            {t('members.remove')}
          </Button>
        </Group>
      </Modal>
    </>
  )
}

function AddMember({ opened, onClose, onAdded }: { opened: boolean; onClose: () => void; onAdded: (m: Member) => void }) {
  const { t } = useI18n()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('recruiter')
  const close = () => {
    setEmail('')
    setPassword('')
    setRole('recruiter')
    onClose()
  }
  const create = useMutation({
    mutationFn: () => createMember(email.trim(), password, role),
    onSuccess: (m) => {
      onAdded(m)
      close()
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })
  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate()
  }
  return (
    <Modal opened={opened} onClose={close} title={t('members.add')} centered>
      <form onSubmit={submit}>
        <Stack>
          <TextInput label={t('signIn.email')} type="email" required value={email} onChange={(e) => setEmail(e.currentTarget.value)} />
          <PasswordInput
            label={t('signIn.temporaryPassword')}
            description={t('members.temporaryHint')}
            required
            value={password}
            onChange={(e) => setPassword(e.currentTarget.value)}
          />
          <div>
            <Text size="sm" fw={500} mb={4}>
              {t('members.col.role')}
            </Text>
            <SegmentedControl
              value={role}
              onChange={(v) => setRole(v as Role)}
              data={[
                { value: 'recruiter', label: t('role.recruiter') },
                { value: 'admin', label: t('role.admin') },
              ]}
            />
          </div>
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" loading={create.isPending} disabled={!email.trim() || password.length < MIN_PASSWORD}>
              {t('members.add')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}

function ResetPassword({ member, onClose, onReset }: { member: Member | null; onClose: () => void; onReset: (m: Member) => void }) {
  const { t } = useI18n()
  const [password, setPassword] = useState('')
  const close = () => {
    setPassword('')
    onClose()
  }
  const reset = useMutation({
    mutationFn: () => setMemberPassword(member!.id, password),
    onSuccess: (m) => {
      onReset(m)
      close()
    },
    onError: (err) => notifications.show({ color: 'red', message: err.message }),
  })
  return (
    <Modal opened={member !== null} onClose={close} title={t('members.resetTitle', { email: member?.email ?? '' })} centered>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          reset.mutate()
        }}
      >
        <Stack>
          <Text size="sm">{t('members.resetBody')}</Text>
          <PasswordInput
            label={t('signIn.temporaryPassword')}
            description={t('members.temporaryHint')}
            required
            value={password}
            onChange={(e) => setPassword(e.currentTarget.value)}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" loading={reset.isPending} disabled={password.length < MIN_PASSWORD}>
              {t('members.resetPassword')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
