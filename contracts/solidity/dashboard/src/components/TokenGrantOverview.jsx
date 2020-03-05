import React, { useState, useEffect } from 'react'
import { tokenGrantsService } from '../services/token-grants.service'
import { useFetchData } from '../hooks/useFetchData'
import { formatDate, displayAmount } from '../utils'
import Dropdown from './Dropdown'
import ProgressBar from './ProgressBar'
import { colors } from '../constants/colors'
import SelectedGrantDropdown from './SelectedGrantDropdown'
import { useModal } from '../hooks/useModal'
import TokenGrantVestingSchedule from './TokenGrantVestingSchedule'

const TokenGrantOverview = (props) => {
  const [state] = useFetchData(tokenGrantsService.fetchGrants, [])
  const { data } = state
  const [selectedGrant, setSelectedGrant] = useState({})
  const { showModal, ModalComponent } = useModal()

  useEffect(() => {
    if (selectedGrant && data.length > 0) {
      setSelectedGrant(data[0])
    }
  }, [data])

  const onSelect = (selectedItem) => {
    setSelectedGrant(selectedItem)
  }

  return (
    <div className="token-grant-overview" style={data.length === 0 ? { display: 'none' } : {}}>
      <ModalComponent title="Vesting Schedule Summary">
        <TokenGrantVestingSchedule grant={selectedGrant} />
      </ModalComponent>
      {
        data.length > 1 &&
        <Dropdown
          onSelect={onSelect}
          options={data}
          valuePropertyName='id'
          labelPropertyName='id'
          selectedItem={selectedGrant}
          labelPrefix='Grant ID'
          noItemSelectedText='Select Grant'
          label="Choose Grant"
          selectedItemComponent={<SelectedGrantDropdown grant={selectedGrant} />}
        />
      }
      <div className="flex row center space-between">
        <h4 className="balance">{displayAmount(selectedGrant.amount)}&nbsp;KEEP</h4>
        <span className="text-warning text-link" onClick={showModal}>Vesting Schedule</span>
      </div>
      <div>
        <ProgressBar
          total={selectedGrant.amount}
          items={[
            { value: selectedGrant.vested, color: colors.grey, label: 'Vested' },
            { value: selectedGrant.released, color: colors.primary, label: 'Relesed' },
            { value: selectedGrant.staked, color: colors.brown, label: 'Staked' },
          ]}
          withLegend
        />
      </div>
    </div>
  )
}

export default TokenGrantOverview