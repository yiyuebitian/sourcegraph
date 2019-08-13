import React from 'react'
import * as GQL from '../../../../shared/src/graphql/schema'
import { ActorLink } from '../../actor/ActorLink'

export interface ParticipantListItemContext {
    showRepository?: boolean
}

interface Props extends ParticipantListItemContext {
    participant: GQL.IParticipantEdge

    className?: string
}

const REASON_DESCRIPTION: Record<GQL.ParticipantReason, string> = {
    CODE_OWNER: 'code owner',
}

/**
 * An item in a list of participants.
 */
export const ParticipantListItem: React.FunctionComponent<Props> = ({ participant, className = '' }) => (
    <li className={`list-group-item d-flex align-items-center ${className}`}>
        <ActorLink actor={participant.actor} />
        {participant.reasons.length > 0 && (
            <ul className="list-inline ml-3">
                {participant.reasons.map((reason, i) => (
                    <li key={i} className="list-inline-item badge badge-secondary mr-2">
                        {REASON_DESCRIPTION[reason]}
                    </li>
                ))}
            </ul>
        )}
    </li>
)