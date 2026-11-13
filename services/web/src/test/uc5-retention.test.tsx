import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { db } from '../mocks/db'
import { renderSignedIn } from './render'

describe('enforcing candidate data retention (UC-5)', () => {
  it('lets a Recruiter keep a held expiry, and shows them only that tab (FR-5.6)', async () => {
    const { user } = await renderSignedIn('/retention')
    expect(screen.queryByRole('tab', { name: 'Policy' })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: 'Keep cd_seed_2 for now' }))
    expect(await screen.findByText('No expiry is waiting for a decision.')).toBeInTheDocument()
    expect(db.results.some((r) => r.candidateId === 'cd_seed_2')).toBe(true)
  })

  it('erases a held candidate everywhere after confirmation, leaving an audit entry (FR-5.3, FR-5.5)', async () => {
    const { user } = await renderSignedIn('/retention', 'admin')
    await user.click(await screen.findByRole('button', { name: 'Erase' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Erase permanently' }))
    expect(await screen.findByText(/^Erased\. Audit reference/)).toBeInTheDocument()
    expect(db.results.some((r) => r.candidateId === 'cd_seed_2')).toBe(false)

    await user.click(screen.getByRole('tab', { name: 'Erasures' }))
    const table = await screen.findByRole('table')
    expect(within(table).getByText('Held, then erased')).toBeInTheDocument()
    // The audit entry names no one once erased.
    expect(within(table).queryByText('cd_seed_2')).not.toBeInTheDocument()
  })

  it('has no policy until an Admin sets one, then saves it (FR-5.1)', async () => {
    const { user } = await renderSignedIn('/retention?tab=policy', 'admin')
    expect(await screen.findByText(/No retention policy is set/)).toBeInTheDocument()
    await user.click(screen.getByText('Anonymise'))
    await user.click(screen.getByRole('button', { name: 'Save policy' }))
    expect(await screen.findByText('Retention policy saved.')).toBeInTheDocument()
    expect(db.policy).toMatchObject({ period_days: 365, expiry_action: 'anonymise', anchor: 'last_activity' })
  })

  it('erases a named candidate on request (FR-5.7)', async () => {
    const { user } = await renderSignedIn('/retention?tab=erasures', 'admin')
    await user.type(await screen.findByLabelText(/Erase a candidate now/), 'cd_seed_3')
    await user.click(screen.getByRole('button', { name: 'Erase' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Erase permanently' }))
    expect(await screen.findByRole('cell', { name: 'Requested' })).toBeInTheDocument()
    expect(db.results.some((r) => r.candidateId === 'cd_seed_3')).toBe(false)
  })

  it("shows the gateway's refusals in the access audit log (FR-0.4)", async () => {
    await renderSignedIn('/retention?tab=audit', 'admin')
    expect(await screen.findByText('GET /members')).toBeInTheDocument()
  })
})
