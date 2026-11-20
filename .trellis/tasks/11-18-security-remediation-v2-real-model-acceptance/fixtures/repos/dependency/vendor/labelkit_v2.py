"""SYNTHETIC ONLY: bounded revision with a documented result-field change."""

from dataclasses import dataclass


@dataclass(frozen=True)
class LabelResult:
    text: str


def format_label(value: str) -> LabelResult:
    text = value.strip()
    if len(text) > 8:
        raise ValueError("Synthetic labels must contain at most eight characters")
    return LabelResult(text=text)
