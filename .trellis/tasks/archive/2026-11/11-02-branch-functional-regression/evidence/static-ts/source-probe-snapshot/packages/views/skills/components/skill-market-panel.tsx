"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspacePaths } from "@multica/core/paths";
import { skillListOptions } from "@multica/core/workspace/queries";
import { toast } from "sonner";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { CreateSkillDialog, type SkillCreateEntry } from "./create-skill-dialog";
import { SkillTemplateCatalog } from "./skill-template-catalog";

interface SkillMarketPanelProps {
  workspaceId: string;
  active?: boolean;
}

/** Keep this panel mounted while switching market types so its draft survives. */
export function SkillMarketPanel(props: SkillMarketPanelProps) {
  return <SkillMarketPanelContent key={props.workspaceId} {...props} />;
}

function SkillMarketPanelContent({ workspaceId, active = true }: SkillMarketPanelProps) {
  const { t } = useT("skills");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const skills = useQuery({ ...skillListOptions(workspaceId), enabled: active && !!workspaceId });
  const opener = useRef<HTMLButtonElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const [creation, setCreation] = useState<SkillCreateEntry | null>(null);

  return (
    <>
      <SkillTemplateCatalog
        workspaceId={workspaceId}
        active={active}
        poll={!creation}
        skills={skills.data}
        skillsError={skills.isError}
        searchRef={searchRef}
        onPreview={(templateName, trigger) => {
          opener.current = trigger;
          setCreation({ kind: "templates", templateName });
        }}
        onCreate={(trigger) => {
          opener.current = trigger;
          setCreation({ kind: "methods" });
        }}
      />
      {creation && (
        <CreateSkillDialog
          active={active}
          initialEntry={creation}
          finalFocus={() => opener.current?.isConnected ? opener.current : searchRef.current}
          onClose={() => setCreation(null)}
          onCreated={(skill) => navigation.push(paths.skillDetail(skill.id))}
          onTemplateCreated={creation.kind === "templates" ? (skill) => {
            toast.success(t(($) => $.create.template.created), {
              action: {
                label: t(($) => $.market.open_created),
                onClick: () => navigation.push(paths.skillDetail(skill.id)),
              },
            });
          } : undefined}
        />
      )}
    </>
  );
}
