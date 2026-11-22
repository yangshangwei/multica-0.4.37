"use client";

import { useEffect, useSyncExternalStore, type ReactNode } from "react";
import { useModalStore, type WorkspaceIssueCreationIntent } from "@multica/core/modals";
import { getApi } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useCurrentWorkspace } from "@multica/core/paths";
import { getCurrentSlug, getCurrentWsId, subscribeToCurrentSlug } from "@multica/core/platform";
import { openCreateIssueWithPreference } from "@multica/core/issues/stores/create-mode-store";
import { currentPath, useNavigation } from "../navigation";
import { CreateIssueDialog } from "./create-issue-dialog";
import { CreateProjectModal } from "./create-project";
import { FeedbackModal } from "./feedback";
import { SetParentIssueModal } from "./set-parent-issue";
import { AddChildIssueModal } from "./add-child-issue";
import { DeleteIssueConfirmModal } from "./delete-issue-confirm";
import { RunConfirmModal } from "./run-confirm";
import { IssueCompletionHintModal } from "./issue-completion-hint";
import { IssueCompletionAuthorizationModal } from "./issue-completion-authorization";
import { IssueLimitUpgradeDialog } from "./issue-limit-upgrade-dialog";

export function ModalRegistry() {
  const modal = useModalStore((s) => s.modal);
  const data = useModalStore((s) => s.data);
  const close = useModalStore((s) => s.close);
  const workspaceIssueCreation = useModalStore((s) => s.workspaceIssueCreation);

  let activeModal: ReactNode = null;
  switch (modal) {
    // Both modal types open the same shell so the in-modal mode switch is
    // instant — only the inner panel swaps, the Dialog Root stays mounted.
    case "create-issue":
      activeModal = (
        <CreateIssueDialog
          onClose={close}
          initialMode="manual"
          data={data}
        />
      );
      break;
    case "quick-create-issue":
      activeModal = (
        <CreateIssueDialog
          onClose={close}
          initialMode="agent"
          data={data}
        />
      );
      break;
    case "create-project":
      activeModal = <CreateProjectModal onClose={close} data={data} />;
      break;
    case "feedback":
      activeModal = <FeedbackModal onClose={close} data={data} />;
      break;
    case "issue-set-parent":
      activeModal = <SetParentIssueModal onClose={close} data={data} />;
      break;
    case "issue-add-child":
      activeModal = <AddChildIssueModal onClose={close} data={data} />;
      break;
    case "issue-delete-confirm":
      activeModal = <DeleteIssueConfirmModal onClose={close} data={data} />;
      break;
    case "issue-run-confirm":
      activeModal = <RunConfirmModal onClose={close} data={data} />;
      break;
    case "issue-completion-hint":
      activeModal = <IssueCompletionHintModal onClose={close} data={data} />;
      break;
    case "issue-completion-authorization":
      activeModal = <IssueCompletionAuthorizationModal onClose={close} data={data} />;
      break;
  }

  return (
    <>
      {workspaceIssueCreation && <WorkspaceIssueCreationHandoff intent={workspaceIssueCreation} />}
      {activeModal}
      <IssueLimitUpgradeDialog />
    </>
  );
}

/** Only this real workspace host may admit an account-side creation intent. */
function WorkspaceIssueCreationHandoff({ intent }: { intent: WorkspaceIssueCreationIntent }) {
  const workspace = useCurrentWorkspace();
  const navigation = useNavigation();
  const path = currentPath(navigation);
  const userId = useAuthStore((state) => state.user?.id);
  const status = useAuthStore((state) => state.status);
  const currentSlug = useSyncExternalStore(subscribeToCurrentSlug, getCurrentSlug, () => null);

  useEffect(() => {
    const client = getApi();
    const viewer = useAuthStore.getState();
    const ready = workspace?.id === getCurrentWsId() && workspace?.slug === currentSlug;
    const consumed = useModalStore.getState().reconcileWorkspaceIssueCreation(intent, {
      api: client,
      sessionScope: client.getSessionScope(),
      userId: viewer.status === "authenticated" ? viewer.user?.id ?? null : null,
      path,
      workspaceId: ready ? workspace?.id : null,
      workspaceSlug: ready ? workspace?.slug : null,
    });
    if (consumed) openCreateIssueWithPreference();
  }, [intent, path, workspace?.id, workspace?.slug, currentSlug, userId, status]);

  return null;
}

/** Account shells can offer feedback without mounting workspace-only modals. */
export function FeedbackModalHost() {
  const modal = useModalStore((s) => s.modal);
  const data = useModalStore((s) => s.data);
  const close = useModalStore((s) => s.close);

  return modal === "feedback" ? <FeedbackModal onClose={close} data={data} /> : null;
}
