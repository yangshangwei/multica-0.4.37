"""Synthetic consumer; its public return type stays str across the upgrade."""

import importlib
import json
from pathlib import Path


def render_label(value: str) -> str:
    lock = json.loads(Path(__file__).with_name("dependency.lock.json").read_text())
    module_name = {
        "1.0.0": "vendor.labelkit_v1",
        "2.0.0": "vendor.labelkit_v2",
    }[lock["version"]]
    dependency = importlib.import_module(module_name)
    return dependency.format_label(value).label
