"""SYNTHETIC ONLY: affected revision of the fictional pilot-labelkit package."""

from dataclasses import dataclass


@dataclass(frozen=True)
class LabelResult:
    label: str


def format_label(value: str) -> LabelResult:
    return LabelResult(label=value.strip())
