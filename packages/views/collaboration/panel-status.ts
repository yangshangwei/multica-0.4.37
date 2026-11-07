/** Ephemeral presentation state for a panel's compact navigation entry. */
export interface CollaborationPanelStatus {
  attention: boolean;
  count?: number;
}

export type CollaborationPanelStatusChange = (status: CollaborationPanelStatus) => void;
