"use client";

import { useCallback } from "react";
import { useT } from "../../i18n";
import {
  getSkillPresentation,
  type SkillPresentationInput,
} from "../lib/skill-presentation";

export function useSkillPresentation() {
  const { t } = useT("skills");
  return useCallback(
    (skill: SkillPresentationInput) => getSkillPresentation(skill, t),
    [t],
  );
}
