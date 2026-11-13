import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderSignedIn } from './render'

describe('monitoring the hiring pipeline (UC-4)', () => {
  it('shows each open or paused opening with its counts per stage, and leaves closed ones out (FR-4.1)', async () => {
    await renderSignedIn('/pipeline')
    const row = (await screen.findByRole('link', { name: 'Backend Engineer' })).closest('tr')!
    // The seed batch: 7 resumes, 6 screened (one unreadable), the best one shortlisted.
    expect(within(row).getAllByRole('cell').map((c) => c.textContent)).toEqual(expect.arrayContaining(['7', '6', '1']))
    expect(screen.queryByText('Data Analyst')).not.toBeInTheDocument()
    // A Recruiter sees the dashboard but cannot run the sweep, which is the Admin's.
    expect(screen.queryByRole('button', { name: 'Check staleness now' })).not.toBeInTheDocument()
  })

  it("flags an opening open longer than its expected time-to-fill when an Admin runs the check (FR-4.3)", async () => {
    const { user } = await renderSignedIn('/pipeline', 'admin')
    const row = (await screen.findByRole('link', { name: 'Backend Engineer' })).closest('tr')!
    expect(within(row).getByText('On track')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Check staleness now' }))
    expect(await within(row).findByText('Stale')).toBeInTheDocument()
  })
})
